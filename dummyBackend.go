package main

import (
	"crypto/sha1"
	"encoding/base64"
)

type DummyBackend struct{}

func (dummy *DummyBackend) Play(song Song) {
}

func (dummy *DummyBackend) Pause() {
}

func (dummy *DummyBackend) Search(keyword string) []Song {

	s := NewDummySong("Dummy Mc Crashtest", "exactly "+keyword)
	return []Song{s}
}

func NewDummySong(title string, artist string) Song {
	s := Song{title, artist, "dummy", "", 0, 0, map[string]struct{}{}, map[string]struct{}{}}
	h := sha1.New()
	h.Write([]byte(title))
	h.Write([]byte(artist))
	s.Uri = base64.URLEncoding.EncodeToString(h.Sum(nil))
	return s
}
