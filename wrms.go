package main

import (
	"encoding/json"
	"sync"
	"sync/atomic"

	"muhq.space/go/wrms/llog"

	"github.com/google/uuid"

	"nhooyr.io/websocket"
)

type Song struct {
	Title     string                 `json:"title"`
	Artist    string                 `json:"artist"`
	Source    string                 `json:"source"`
	Uri       string                 `json:"uri"`
	Weight    int                    `json:"weight"`
	index     int                    `json:"-"` // used by heap.Interface
	Upvotes   map[uuid.UUID]struct{} `json:"-"`
	Downvotes map[uuid.UUID]struct{} `json:"-"`
}

func NewSong(title, artist, source, uri string) Song {
	return Song{title, artist, source, uri, 0, 0, map[uuid.UUID]struct{}{}, map[uuid.UUID]struct{}{}}
}

func NewSongFromJson(data []byte) (Song, error) {
	var s Song
	err := json.Unmarshal(data, &s)
	if err != nil {
		llog.Error("Failed to parse song data %s with %s", string(data), err)
		return s, err
	}

	s.Upvotes = map[uuid.UUID]struct{}{}
	s.Downvotes = map[uuid.UUID]struct{}{}
	return s, nil
}

type Event struct {
	Event string `json:"cmd"`
	Id    int    `json:"id"`
	Songs []Song `json:"songs"`
}

func newEvent(event string, songs []Song) Event {
	return Event{Event: event, Songs: songs}
}

func newNotification(notification string) Event {
	return Event{Event: notification}
}

func newSearchResultEvent(id int, songs []Song) Event {
	return Event{Event: "search", Id: id, Songs: songs}
}

type Connection struct {
	Id      uuid.UUID
	closing atomic.Bool
	refs    atomic.Int64
	Events  chan Event
	C       *websocket.Conn
}

const EVENT_BUFFER_SIZE = 3

func newConnection(id uuid.UUID, ws *websocket.Conn) *Connection {
	return &Connection{
		Id:     id,
		Events: make(chan Event, EVENT_BUFFER_SIZE),
		C:      ws,
	}
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
	Songs       []Song
	queue       Playlist
	CurrentSong Song
	Player      Player
	Playing     bool
	Config      Config
}

func NewWrms(config Config) *Wrms {
	wrms := Wrms{}
	wrms.Config = config
	wrms.Player = NewPlayer(&wrms, config.Backends)
	return &wrms
}

func (wrms *Wrms) addConn(conn *Connection) {
	llog.DDebug("Adding Connection %s", conn.Id)
	wrms.Connections.Store(conn.Id, conn)
}

func (wrms *Wrms) delConn(conn *Connection) {
	llog.DDebug("Deleting Connection %s", conn.Id)
	wrms.Connections.Delete(conn.Id)
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

func (wrms *Wrms) startPlaying() {
	wrms.Broadcast(newEvent("play", []Song{wrms.CurrentSong}))
	wrms.Player.Play(&wrms.CurrentSong)
}

func (wrms *Wrms) AddSong(song Song) {
	startPlayingAgain := wrms.Playing && wrms.CurrentSong.Uri == ""
	wrms.Songs = append(wrms.Songs, song)
	s := &wrms.Songs[len(wrms.Songs)-1]
	wrms.queue.Add(s)
	llog.Info("Added song %s (ptr=%p) to Songs", s.Uri, s)
	wrms.Broadcast(newEvent("add", []Song{song}))

	if startPlayingAgain {
		wrms.Next()
	}
}

func (wrms *Wrms) DeleteSong(songUri string) {
	for i := 0; i < len(wrms.Songs); i++ {
		s := &wrms.Songs[i]
		if s.Uri != songUri {
			continue
		}

		wrms.Songs[i] = wrms.Songs[len(wrms.Songs)-1]
		wrms.Songs = wrms.Songs[:len(wrms.Songs)-1]

		wrms.queue.RemoveSong(s)
		wrms.Broadcast(newEvent("delete", []Song{*s}))
		break
	}
}

func (wrms *Wrms) Next() {
	llog.DDebug("Next Song")

	next := wrms.queue.PopSong()
	if next == nil {
		wrms.CurrentSong.Uri = ""
		wrms.Broadcast(newNotification("stop"))
		return
	}

	wrms.CurrentSong = *next

	llog.Info("popped next song and removing it from the song list %v", next)

	for i, s := range wrms.Songs {
		if s.Uri == wrms.CurrentSong.Uri {
			wrms.Songs[i] = wrms.Songs[len(wrms.Songs)-1]
			wrms.Songs = wrms.Songs[:len(wrms.Songs)-1]
			break
		}
	}

	if wrms.Playing {
		wrms.startPlaying()
	}
}

func (wrms *Wrms) PlayPause() {
	wrms.Playing = !wrms.Playing

	if wrms.Playing {
		if wrms.CurrentSong.Uri == "" {
			llog.Info("No song currently playing play the next")
			wrms.Next()
		} else {
			wrms.startPlaying()
		}
	} else {
		wrms.Broadcast(newNotification("pause"))
		wrms.Player.Pause()
	}
}

func (wrms *Wrms) AdjustSongWeight(connId uuid.UUID, songUri string, vote string) {
	for i := 0; i < len(wrms.Songs); i++ {
		s := &wrms.Songs[i]
		if s.Uri != songUri {
			continue
		}

		llog.Info("Adjusting song %s (ptr=%p)", s.Uri, s)
		switch vote {
		case "up":
			if _, ok := s.Upvotes[connId]; ok {
				llog.Error("Double upvote of song %s by connections %s", songUri, connId)
				return
			}

			if _, ok := s.Downvotes[connId]; ok {
				delete(s.Downvotes, connId)
				s.Weight += 2
			} else {
				s.Weight += 1
			}

			s.Upvotes[connId] = struct{}{}

		case "down":
			if _, ok := s.Downvotes[connId]; ok {
				llog.Error("Double downvote of song %s by connections %s", songUri, connId)
				return
			}

			if _, ok := s.Upvotes[connId]; ok {
				delete(s.Upvotes, connId)
				s.Weight -= 2
			} else {
				s.Weight -= 1
			}

			s.Downvotes[connId] = struct{}{}

		case "unvote":
			if _, ok := s.Downvotes[connId]; ok {
				delete(s.Downvotes, connId)
				s.Weight += 1
			}

			if _, ok := s.Upvotes[connId]; ok {
				delete(s.Upvotes, connId)
				s.Weight -= 1
			} else {
				llog.Error("Double unvote of song %s by connections %s", songUri, connId)
				return
			}

		default:
			llog.Fatal("invalid vote")
		}

		wrms.queue.Adjust(s)
		wrms.Broadcast(newEvent("update", []Song{*s}))
		break
	}
}
