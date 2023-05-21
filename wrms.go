package main

import (
	"encoding/json"

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
	Songs []Song `json:"songs"`
}

func newNotification(notification string) Event {
	return Event{notification, nil}
}

type Connection struct {
	Id     uuid.UUID
	Events chan Event
	C      *websocket.Conn
}

func (c *Connection) Send(ev Event) {
	c.Events <- ev
}

func (c *Connection) Close() {
	llog.Info("Closing connection %s", c.Id)
	close(c.Events)
	c.C.Close(websocket.StatusNormalClosure, "")
}

type Wrms struct {
	Connections map[uuid.UUID]*Connection
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
	wrms.Connections = make(map[uuid.UUID]*Connection)
	wrms.Player = NewPlayer(&wrms, config.Backends)
	return &wrms
}

func (wrms *Wrms) GetConn(connId uuid.UUID) *Connection {
	return wrms.Connections[connId]
}

func (wrms *Wrms) Close() {
	llog.Info("Closing WRMS")
	for _, conn := range wrms.Connections {
		conn.Close()
	}
}

func (wrms *Wrms) Broadcast(ev Event) {
	llog.Info("Broadcasting %v", ev)
	for _, conn := range wrms.Connections {
		conn.Send(ev)
	}
}

func (wrms *Wrms) AddSong(song Song) {
	wrms.Songs = append(wrms.Songs, song)
	s := &wrms.Songs[len(wrms.Songs)-1]
	wrms.queue.Add(s)
	llog.Info("Added song %s (ptr=%p) to Songs", s.Uri, s)
}

func (wrms *Wrms) Next() {
	llog.DDebug("Next Song")

	next := wrms.queue.PopSong()
	if next == nil {
		wrms.CurrentSong.Uri = ""
		return
	}

	llog.Info("popped next song and removing it from the song list %v", next)

	for i, s := range wrms.Songs {
		if s.Uri == wrms.CurrentSong.Uri {
			wrms.Songs[i] = wrms.Songs[len(wrms.Songs)-1]
			wrms.Songs = wrms.Songs[:len(wrms.Songs)-1]
			break
		}
	}

	wrms.CurrentSong = *next
}

func (wrms *Wrms) PlayPause() {
	var ev Event
	if !wrms.Playing {
		if wrms.CurrentSong.Uri == "" {
			llog.Info("No song currently playing play the next")
			wrms.Next()
			if wrms.CurrentSong.Uri == "" {
				llog.Info("No song left to play")
				wrms.Broadcast(newNotification("stop"))
				return
			}
		}
		ev = Event{"play", []Song{wrms.CurrentSong}}
		wrms.Player.Play(&wrms.CurrentSong)
	} else {
		ev = newNotification("pause")
		wrms.Player.Pause()
	}

	wrms.Playing = !wrms.Playing

	wrms.Broadcast(ev)
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
		wrms.Broadcast(Event{"update", []Song{*s}})
		break
	}
}
