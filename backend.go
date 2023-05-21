package main

import (
	"crypto/sha1"
	"encoding/base64"
)

type Backend interface {
	Play(song *Song, player *Player)
	Search(keyword string) []Song
	OnSongFinished(song *Song)
}

type DummyBackend struct{}

func (dummy *DummyBackend) Play(song *Song, player *Player) {}
func (dummy *DummyBackend) OnSongFinished(song *Song)       {}
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
