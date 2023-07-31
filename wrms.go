package main

import (
	"sync"
	"sync/atomic"

	"muhq.space/go/wrms/llog"

	"github.com/google/uuid"
)

type Event struct {
	Event string  `json:"cmd"`
	Id    uint64  `json:"id"`
	Songs []*Song `json:"songs"`
}

func (wrms *Wrms) incEventId() uint64 {
	id := wrms.eventId.Add(1)
	llog.DDebug("Increment event id to: %d", id)
	return id
}

// A private event does not appear in the global event order and
// thus does not increment the global event count.
// Private events are used for example to report search results.
func (wrms *Wrms) newPrivateEvent(id uint64, event string, songs []*Song) Event {
	return Event{Id: id, Event: event, Songs: songs}
}

func (wrms *Wrms) newEvent(event string, songs []*Song) Event {
	return Event{Id: wrms.incEventId(), Event: event, Songs: songs}
}

func (wrms *Wrms) newNotification(notification string) Event {
	return wrms.newEvent(notification, nil)
}

type Wrms struct {
	Connections sync.Map
	nextConnNr  atomic.Uint64
	// The rwlock must be held when using most internal state
	rwlock      sync.RWMutex
	Songs       []*Song
	queue       Playlist
	CurrentSong atomic.Pointer[Song]
	Player      Player
	playing     bool
	Config      Config
	eventId     atomic.Uint64
}

func NewWrms(config Config) *Wrms {
	wrms := Wrms{}
	wrms.Config = config
	wrms.Player = NewMpvPlayer(&wrms, config.Backends)

	if len(wrms.Config.Playlists) > 0 {
		wrms.loadPlaylists(wrms.Config.Playlists)
	}

	return &wrms
}

func (wrms *Wrms) addConn(conn *Connection) {
	llog.DDebug("Adding Connection %s", conn.Id)
	wrms.Connections.Store(conn.Id, conn)
}

func (wrms *Wrms) delConn(conn *Connection) {
	llog.DDebug("Deleting Connection %s", conn.Id)
	_oldConn, _ := wrms.Connections.Load(conn.Id)
	oldConn := _oldConn.(*Connection)
	// Only delete the connection if no newer one is currently active
	if oldConn.nr == conn.nr {
		wrms.Connections.Delete(conn.Id)
	}
}

func (wrms *Wrms) initConn(conn *Connection) error {
	wrms.rwlock.RLock()

	curEventId := wrms.eventId.Load()
	conn.nextEvent = curEventId + 1

	initialCmds := []interface{}{}

	if wrms.Config.timeBonus != 0 {
		ev := map[string]any{"cmd": "timeBonus", "timeBonus": wrms.Config.timeBonus}
		initialCmds = append(initialCmds, ev)
	}

	if wrms.playing {
		var songs []*Song
		if currentSong := wrms.CurrentSong.Load(); currentSong != nil {
			songs = []*Song{currentSong}
		}
		initialCmds = append(initialCmds, wrms.newPrivateEvent(curEventId, "play", songs))
	}

	upvoted := []*Song{}
	downvoted := []*Song{}
	if len(wrms.Songs) > 0 {
		initialCmds = append(initialCmds, wrms.newPrivateEvent(curEventId, "add", wrms.Songs))

		for _, song := range wrms.queue.OrderedList() {
			llog.DDebug("Looking at the votes of song: %v", song)
			if _, ok := song.Upvotes[conn.Id]; ok {
				upvoted = append(upvoted, song)
			} else if _, ok := song.Downvotes[conn.Id]; ok {
				downvoted = append(downvoted, song)
			}
		}
	}

	if len(upvoted) > 0 {
		initialCmds = append(initialCmds, wrms.newPrivateEvent(curEventId, "upvoted", upvoted))
	}

	if len(downvoted) > 0 {
		initialCmds = append(initialCmds, wrms.newPrivateEvent(curEventId, "downvoted", downvoted))
	}

	wrms.rwlock.RUnlock()
	llog.Info("Sending initial cmds %v", initialCmds)
	return conn._send(initialCmds)
}

func (wrms *Wrms) GetConn(connId uuid.UUID) *Connection {
	conn, _ := wrms.Connections.Load(connId)
	return conn.(*Connection)
}

func (wrms *Wrms) Broadcast(ev Event) {
	llog.Info("Broadcasting %v", ev)
	wrms.Connections.Range(func(_, conn any) bool {
		conn.(*Connection).Send(ev)
		return true
	})
}

func (wrms *Wrms) _addSong(song *Song) {
	wrms.Songs = append(wrms.Songs, song)
	wrms.queue.Add(song)
}

func (wrms *Wrms) AddSong(song *Song) {
	wrms.rwlock.Lock()

	startPlayingAgain := wrms.playing && wrms.CurrentSong.Load() == nil

	if wrms.Config.timeBonus != 0 {
		llog.Info("Apply time bonus %v", wrms.Config.timeBonus)
		wrms.queue.applyTimeBonus(wrms.Config.timeBonus)
	}

	wrms._addSong(song)

	ev := wrms.newEvent("add", []*Song{song})
	wrms.rwlock.Unlock()

	llog.Info("Added song %s (ptr=%p) to Songs", song.Uri, song)
	wrms.Broadcast(ev)

	if startPlayingAgain {
		wrms._lockedNext()
	}
}

func (wrms *Wrms) DeleteSong(songUri string) {
	wrms.rwlock.Lock()

	for i := 0; i < len(wrms.Songs); i++ {
		s := wrms.Songs[i]
		if s.Uri != songUri {
			continue
		}

		wrms.Songs[i] = wrms.Songs[len(wrms.Songs)-1]
		wrms.Songs = wrms.Songs[:len(wrms.Songs)-1]

		wrms.queue.RemoveSong(s)

		ev := wrms.newEvent("delete", []*Song{s})
		wrms.rwlock.Unlock()

		wrms.Broadcast(ev)
		break
	}
}

func (wrms *Wrms) Next() {
	wrms.rwlock.Lock()

	// Terminate the Player if it is currently playing
	if wrms.Player.Playing() {
		wrms.Player.Stop()
	}

	wrms._next()
}

func (wrms *Wrms) _lockedNext() {
	wrms.rwlock.Lock()
	wrms._next()
}

func (wrms *Wrms) _next() {
	llog.DDebug("Next Song")

	next := wrms.queue.PopSong()
	if next == nil {
		wrms.CurrentSong.Store(nil)
		wrms.Broadcast(wrms.newNotification("stop"))
		wrms.rwlock.Unlock()
		return
	}

	wrms.CurrentSong.Store(next)

	llog.Info("popped next song and removing it from the song list %v", next)

	for i, s := range wrms.Songs {
		if s.Uri == next.Uri {
			wrms.Songs[i] = wrms.Songs[len(wrms.Songs)-1]
			wrms.Songs = wrms.Songs[:len(wrms.Songs)-1]
			break
		}
	}

	cmd := "next"
	// We are playing -> start playing the next song
	if wrms.playing {
		wrms.Player.Play(next)
		cmd = "play"
	}

	ev := wrms.newEvent(cmd, []*Song{next})
	wrms.rwlock.Unlock()

	wrms.Broadcast(ev)
}

func (wrms *Wrms) PlayPause() {
	wrms.rwlock.Lock()
	// Toggle the playback state
	wrms.playing = !wrms.playing

	// Wrms was playing -> pause the player
	if !wrms.playing {
		wrms.Player.Pause()
		wrms.rwlock.Unlock()
		wrms.Broadcast(wrms.newNotification("pause"))
		return
	}

	// Wrms was not playing
	currentSong := wrms.CurrentSong.Load()

	// Wrms has no current song -> try to play the next song
	if currentSong == nil {
		llog.Info("No song currently playing play the next")
		// _next() releases the rwlock
		wrms._next()
		return
	}

	// The player is playing -> continue playing
	if wrms.Player.Playing() {
		wrms.Player.Continue()
		// The player is stopped -> start it
	} else {
		wrms.Player.Play(currentSong)
	}

	ev := wrms.newEvent("play", []*Song{currentSong})
	wrms.rwlock.Unlock()

	wrms.Broadcast(ev)
}

func (wrms *Wrms) AdjustSongWeight(connId uuid.UUID, songUri string, vote string) {
	wrms.rwlock.Lock()

	for i := 0; i < len(wrms.Songs); i++ {
		s := wrms.Songs[i]
		if s.Uri != songUri {
			continue
		}

		llog.Info("Adjusting song %v (ptr=%p)", s, s)
		switch vote {
		case "up":
			if _, ok := s.Upvotes[connId]; ok {
				llog.Error("Double upvote of song %s by connections %s", songUri, connId)
				return
			}

			if _, ok := s.Downvotes[connId]; ok {
				delete(s.Downvotes, connId)
				s.Weight += 2.0
				llog.Debug("Flip downvote")
			} else {
				s.Weight += 1.0
			}

			s.Upvotes[connId] = struct{}{}

		case "down":
			if _, ok := s.Downvotes[connId]; ok {
				llog.Error("Double downvote of song %s by connections %s", songUri, connId)
				return
			}

			if _, ok := s.Upvotes[connId]; ok {
				delete(s.Upvotes, connId)
				llog.Debug("Flip upvote")
				s.Weight -= 2.0
			} else {
				s.Weight -= 1.0
			}

			s.Downvotes[connId] = struct{}{}

		case "unvote":
			if _, ok := s.Downvotes[connId]; ok {
				delete(s.Downvotes, connId)
				s.Weight += 1.0

			} else if _, ok := s.Upvotes[connId]; ok {
				delete(s.Upvotes, connId)
				s.Weight -= 1.0
			} else {
				llog.Error("Double unvote of song %s by connections %s", songUri, connId)
				return
			}

		default:
			llog.Fatal("invalid vote")
		}

		wrms.queue.Adjust(s)
		ev := wrms.newEvent("update", []*Song{s})
		wrms.rwlock.Unlock()

		wrms.Broadcast(ev)
		break
	}
}

func (wrms *Wrms) Search(pattern map[string]string) chan []*Song {
	return wrms.Player.Search(pattern)
}

func (wrms *Wrms) loadPlaylists(playlists []string) {
	for _, playlist := range playlists {
		wrms.appendPlaylist(playlist)
	}
}

func (wrms *Wrms) appendPlaylist(playlist string) {
	songs := wrms.Player.LoadPlaylist(playlist)

	for _, song := range songs {
		wrms._addSong(song)
	}
}
