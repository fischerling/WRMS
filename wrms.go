package main

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"log"

	"github.com/google/uuid"

	"nhooyr.io/websocket"
)

type Song struct {
	Title     string              `json:"title"`
	Artist    string              `json:"artist"`
	Source    string              `json:"source"`
	Id        string              `json:"id"`
	Weight    int                 `json:"weight"`
	index     int                 `json:"-"` // used by heap.Interface
	Upvotes   map[string]struct{} `json:"-"`
	Downvotes map[string]struct{} `json:"-"`
}

func NewSong(title string, artist string, source string) Song {
	s := Song{title, artist, source, "", 0, 0, map[string]struct{}{}, map[string]struct{}{}}
	h := sha1.New()
	h.Write([]byte(title))
	h.Write([]byte(artist))
	h.Write([]byte(source))
	s.Id = base64.URLEncoding.EncodeToString(h.Sum(nil))
	return s
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
	log.Println("Closing connection %s", c.Id)
	close(c.Events)
	c.C.Close(websocket.StatusNormalClosure, "")
}

type Wrms struct {
	Connections []Connection
	Songs       []Song
	Queue       Playlist
	CurrentSong *Song
	Player      Player
	Playing     bool
}

func NewWrms() Wrms {
	wrms := Wrms{}
	wrms.Player = NewPlayer()
	return wrms
}

func (wrms *Wrms) Close() {
	log.Println("Closing WRMS")
	for _, conn := range wrms.Connections {
		conn.Close()
	}
}

func (wrms *Wrms) Broadcast(cmd Event) {
	for i := 0; i < len(wrms.Connections); i++ {
		wrms.Connections[i].Events <- cmd
	}
}

func (wrms *Wrms) AddSong(song Song) {
	wrms.Songs = append(wrms.Songs, song)
	wrms.Queue.Add(&wrms.Songs[len(wrms.Songs) - 1])
}

func (wrms *Wrms) Next() {
	wrms.CurrentSong = wrms.Queue.PopSong()
	for i, s := range wrms.Songs {
		if s.Id == wrms.CurrentSong.Id {
			wrms.Songs[i] = wrms.Songs[len(wrms.Songs) - 1]
			wrms.Songs = wrms.Songs[:len(wrms.Songs) - 1]
			break
		}
	}
}

func (wrms *Wrms) PlayPause() {
	var ev Event
	if !wrms.Playing {
		if wrms.CurrentSong == nil {
			wrms.Next()
		}
		ev = Event{"play", []Song{*wrms.CurrentSong}}
	} else {
		ev = Event{"pause", []Song{}}
	}

	wrms.Playing = !wrms.Playing

	wrms.Broadcast(ev)
}

func (wrms *Wrms) AdjustSongWeight(connId string, songId string, vote string) {
	for i := 0; i < len(wrms.Songs); i++ {
		s := &wrms.Songs[i]
		if s.Id != songId {
			continue
		}

		switch vote {
		case "up":
			if _, ok := s.Upvotes[connId]; ok {
				log.Println(fmt.Sprintf("Double upvote of song %s by connections %s", songId, connId))
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
				log.Println(fmt.Sprintf("Double downvote of song %s by connections %s", songId, connId))
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
				log.Println(fmt.Sprintf("Double downvote of song %s by connections %s", songId, connId))
				return
			}

		default:
			log.Fatal("invalid vote")
		}

		wrms.Queue.Adjust(s)
		wrms.Broadcast(Event{"update", []Song{*s}})
		break
	}
}
