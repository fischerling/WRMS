package main

import (
	"crypto/sha1"
	"encoding/base64"
)

type DummyBackend struct{}

func (dummy *DummyBackend) Play(song *Song) {
}

func (dummy *DummyBackend) Pause() {
}

func (dummy *DummyBackend) Search(keyword string) []Song {

	s := NewDummySong("Dummy Mc Crashtest", "exactly "+keyword)
	return []Song{s}
}

func NewDummySong(title, artist string) Song {
	h := sha1.New()
	h.Write([]byte(title))
	h.Write([]byte(artist))
	return NewSong(title, artist, "dummy", base64.URLEncoding.EncodeToString(h.Sum(nil)))
}
