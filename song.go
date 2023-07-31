package main

import (
	"encoding/json"

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
