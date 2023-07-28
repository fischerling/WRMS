package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"muhq.space/go/wrms/llog"

	"github.com/google/uuid"
)

type Song struct {
	Title     string                 `json:"title"`
	Artist    string                 `json:"artist"`
	Source    string                 `json:"source"`
	Uri       string                 `json:"uri"`
	Weight    float64                `json:"weight"`
	Album     string                 `json:"album"`
	Year      int                    `json:"year"`
	index     int                    `json:"-"` // used by heap.Interface
	Upvotes   map[uuid.UUID]struct{} `json:"upvotes"`
	Downvotes map[uuid.UUID]struct{} `json:"downvotes"`
}

func NewSong(title, artist, source, uri string) *Song {
	return &Song{
		Title:     title,
		Artist:    artist,
		Source:    source,
		Uri:       uri,
		Upvotes:   map[uuid.UUID]struct{}{},
		Downvotes: map[uuid.UUID]struct{}{},
	}
}

func NewDetailedSong(title, artist, source, uri, album string, year int) *Song {
	s := NewSong(title, artist, source, uri)
	s.Album = album
	s.Year = year
	return s
}

func NewSongFromJson(data []byte) (*Song, error) {
	var s Song
	err := json.Unmarshal(data, &s)
	if err != nil {
		llog.Error("Failed to parse song data %s with %s", string(data), err)
		return nil, err
	}

	s.Upvotes = map[uuid.UUID]struct{}{}
	s.Downvotes = map[uuid.UUID]struct{}{}
	return &s, nil
}

type Event struct {
	Event string  `json:"cmd"`
	Id    uint64  `json:"id"`
	Songs []*Song `json:"songs"`
}

func (wrms *Wrms) newEvent(event string, songs []*Song) Event {
	return Event{Id: wrms.eventId.Add(1), Event: event, Songs: songs}
}

func (wrms *Wrms) newNotification(notification string) Event {
	return wrms.newEvent(notification, nil)
}

type Connection struct {
	wrms      *Wrms
	Id        uuid.UUID
	nr        uint64
	closing   atomic.Bool
	refs      atomic.Int64
	Events    chan Event
	w         http.ResponseWriter
	flusher   http.Flusher
	ctx       context.Context
	cancel    func()
	nextEvent uint64
}

const EVENT_BUFFER_SIZE = 3

func (wrms *Wrms) newConnection(id uuid.UUID,
	w http.ResponseWriter,
	ctx context.Context,
	cncl func()) *Connection {
	// prepare the flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		llog.Fatal("Flusher type assertion failed")
	}

	conn := &Connection{
		wrms:    wrms,
		Id:      id,
		nr:      wrms.nextConnNr.Add(1),
		Events:  make(chan Event, EVENT_BUFFER_SIZE),
		w:       w,
		flusher: flusher,
		ctx:     ctx,
		cancel:  cncl,
	}

	wrms.addConn(conn)
	return conn
}

func (c *Connection) Send(ev Event) {
	llog.DDebug("Sending %v to %v", ev, c.Id)
	// Connection is closing -> not write to it
	if c.closing.Load() {
		return
	}
	// Register us as sender
	c.refs.Add(1)
	// Send the event
	c.Events <- ev
	// Deregister us as sender
	c.refs.Add(-1)
}

func (conn *Connection) serve() {
	// Map to store future events to send them in order
	toSend := make(map[uint64]*Event)

	for {
		var ev Event
		// We have the next event -> send it
		if toSend[conn.nextEvent] != nil {
			ev = *toSend[conn.nextEvent]
			delete(toSend, conn.nextEvent)
			// We have not received the next event yet -> receive a new event
		} else {
			llog.DDebug("%v: Awaiting event %d", conn.Id, conn.nextEvent)
			select {
			case ev = <-conn.Events:
			case <-conn.ctx.Done():
				return
			}
			// We received a future object -> store it and continue
			if ev.Id > conn.nextEvent {
				llog.DDebug("%v: Received future event %d", conn.Id, ev.Id)
				toSend[ev.Id] = &ev
				continue
			}
		}

		conn.nextEvent++

		data, err := json.Marshal(ev)
		if err != nil {
			llog.Error("Encoding the %s event failed with %s", ev.Event, err)
			return
		}

		sdata := string(data)
		llog.Debug("Sending ev %s to %s", sdata, conn.Id)
		fmt.Fprintf(conn.w, "data: %s\n\n", sdata)
		conn.flusher.Flush()
	}
}

func (c *Connection) Close() {
	llog.Info("Closing connection %s", c.Id)
	// Remove the closing connection from the map
	wrms.delConn(c)
	// Announce that the connection is going to be closed
	c.closing.Store(true)

	// Consume all events from the registered senders
	for c.refs.Load() > 0 {
		select {
		case <-c.Events:
		default:
		}
	}

	close(c.Events)
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
	defer wrms.rwlock.RUnlock()

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
		initialCmds = append(initialCmds, wrms.newEvent("play", songs))
	}

	upvoted := []*Song{}
	downvoted := []*Song{}
	if len(wrms.Songs) > 0 {
		initialCmds = append(initialCmds, wrms.newEvent("add", wrms.Songs))

		for _, song := range wrms.queue.OrderedList() {
			if _, ok := song.Upvotes[conn.Id]; ok {
				upvoted = append(upvoted, song)
			} else if _, ok := song.Downvotes[conn.Id]; ok {
				downvoted = append(downvoted, song)
			}
		}
	}

	if len(upvoted) > 0 {
		initialCmds = append(initialCmds, wrms.newEvent("upvoted", upvoted))
	}

	if len(downvoted) > 0 {
		initialCmds = append(initialCmds, wrms.newEvent("downvoted", downvoted))
	}

	for _, cmd := range initialCmds {
		llog.Info("Sending initial cmd %v", cmd)
		data, err := json.Marshal(cmd)
		if err != nil {
			llog.Warning("Encoding the initial command %v failed with %s", cmd, err)
			return err
		}

		fmt.Fprintf(conn.w, "data: %s\n\n", string(data))
		conn.flusher.Flush()
	}

	return nil
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

func (wrms *Wrms) AddSong(song *Song) {
	wrms.rwlock.Lock()

	startPlayingAgain := wrms.playing && wrms.CurrentSong.Load() == nil

	if wrms.Config.timeBonus != 0 {
		llog.Info("Apply time bonus %v", wrms.Config.timeBonus)
		wrms.queue.applyTimeBonus(wrms.Config.timeBonus)
	}

	wrms.Songs = append(wrms.Songs, song)
	s := wrms.Songs[len(wrms.Songs)-1]
	wrms.queue.Add(s)

	ev := wrms.newEvent("add", []*Song{song})
	wrms.rwlock.Unlock()

	llog.Info("Added song %s (ptr=%p) to Songs", s.Uri, s)
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
