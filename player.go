package main

import (
	"log"
)

type Backend interface {
	Play(song Song)
	Pause()
	Search(keyword string) []Song
}

type Player struct {
	Backends map[string]Backend
}

func NewPlayer() Player {
	available_backends := map[string]Backend{}

	spotify, err := NewSpotify()
	if err != nil {
		// log.Fatalln("Error during initialization of the spotify backend:", err.Error())
		log.Println("Error during initialization of the spotify backend:", err.Error())
	}
	available_backends["spotify"] = spotify

	return Player{available_backends}
}

func (player *Player) Play(song Song) {
	player.Backends[song.Source].Play(song)
}

func (player *Player) Pause() {
}

func (player *Player) Search(pattern string) []Song {
	results := []Song{}
	for _, backend := range player.Backends {
		results = append(results, backend.Search(pattern)...)
	}
	return results
}
