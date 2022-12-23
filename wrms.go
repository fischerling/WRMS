package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"muhq.space/go/wrms/llog"

	"github.com/google/uuid"

	"nhooyr.io/websocket"
)

type Song struct {
	Title     string              `json:"title"`
	Artist    string              `json:"artist"`
	Source    string              `json:"source"`
	Uri       string              `json:"uri"`
	Weight    int                 `json:"weight"`
	index     int                 `json:"-"` // used by heap.Interface
	Upvotes   map[string]struct{} `json:"-"`
	Downvotes map[string]struct{} `json:"-"`
}

func NewSong(title, artist, source, uri string) Song {
	return Song{title, artist, source, uri, 0, 0, map[string]struct{}{}, map[string]struct{}{}}
}

func NewSongFromJson(data []byte) (Song, error) {
	var s Song
	err := json.Unmarshal(data, &s)
	if err != nil {
		llog.Error(fmt.Sprintf("Failed to parse song data %s with %s", string(data), err))
		return s, err
	}

	s.Upvotes = map[string]struct{}{}
	s.Downvotes = map[string]struct{}{}
	return s, nil
}

type Event struct {
	Event string `json:"cmd"`
	Songs []Song `json:"songs"`
}

type Connection struct {
	Id     uuid.UUID
	Events chan Event
	C      *websocket.Conn
}

func (c *Connection) Close() {
	llog.Info(fmt.Sprintf("Closing connection %s", c.Id))
	close(c.Events)
	c.C.Close(websocket.StatusNormalClosure, "")
}

type Wrms struct {
	Connections []Connection
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
	wrms.Player = NewPlayer(&wrms, strings.Split(config.backends, " "))
	return &wrms
}

func (wrms *Wrms) Close() {
	llog.Info("Closing WRMS")
	for _, conn := range wrms.Connections {
		conn.Close()
	}
}

func (wrms *Wrms) Broadcast(cmd Event) {
	llog.Info(fmt.Sprintf("Broadcasting %v", cmd))
	for i := 0; i < len(wrms.Connections); i++ {
		wrms.Connections[i].Events <- cmd
	}
}

func (wrms *Wrms) AddSong(song Song) {
	wrms.Songs = append(wrms.Songs, song)
	s := &wrms.Songs[len(wrms.Songs)-1]
	wrms.queue.Add(s)
	llog.Info(fmt.Sprintf("Added song %s (ptr=%p) to Songs", s.Uri, s))
}

func (wrms *Wrms) Next() {
	next := wrms.queue.PopSong()
	if next == nil {
		wrms.CurrentSong.Uri = ""
		return
	}

	llog.Info(fmt.Sprintf("popped next song and removing it from the song list %v", next))

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
				wrms.Broadcast(Event{"stop", []Song{}})
				return
			}
		}
		ev = Event{"play", []Song{wrms.CurrentSong}}
		wrms.Player.Play(&wrms.CurrentSong)
	} else {
		ev = Event{"pause", []Song{}}
		wrms.Player.Pause()
	}

	wrms.Playing = !wrms.Playing

	wrms.Broadcast(ev)
}

func (wrms *Wrms) AdjustSongWeight(connId string, songUri string, vote string) {
	for i := 0; i < len(wrms.Songs); i++ {
		s := &wrms.Songs[i]
		if s.Uri != songUri {
			continue
		}

		llog.Info(fmt.Sprintf("Adjusting song %s (ptr=%p)", s.Uri, s))
		switch vote {
		case "up":
			if _, ok := s.Upvotes[connId]; ok {
				llog.Error(fmt.Sprintf("Double upvote of song %s by connections %s", songUri, connId))
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
				llog.Error(fmt.Sprintf("Double downvote of song %s by connections %s", songUri, connId))
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
				llog.Error(fmt.Sprintf("Double unvote of song %s by connections %s", songUri, connId))
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
