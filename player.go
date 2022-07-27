package main

import (
	"fmt"
	"io"
	"log"
	"os/exec"
	"syscall"
)

type Backend interface {
	Play(song *Song, player *Player)
	Search(keyword string) []Song
}

type Player struct {
	Backends map[string]Backend
	mpv      *exec.Cmd
	wrms     *Wrms
}

func NewPlayer(wrms *Wrms) Player {
	available_backends := map[string]Backend{}

	spotify, err := NewSpotify()
	if err != nil {
		// log.Fatalln("Error during initialization of the spotify backend:", err.Error())
		log.Println("Error during initialization of the spotify backend:", err.Error())
	}
	available_backends["spotify"] = spotify
	available_backends["youtube"] = &YoutubeBackend{}

	available_backends["dummy"] = &DummyBackend{}

	return Player{available_backends, nil, wrms}
}

func (player *Player) Play(song *Song) {
	if player.mpv != nil {
		log.Println("Send SIGCONT to mpv subprocess")
		player.mpv.Process.Signal(syscall.SIGCONT)
		return
	}

	log.Println("Start playing", song)
	player.Backends[song.Source].Play(song, player)
}

func (player *Player) runMpv() {
	_, err := player.mpv.CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("mpv finished. Resetting mpv, the current song and call play")
	player.mpv = nil
	player.wrms.CurrentSong.Uri = ""
	player.wrms.PlayPause()
}

func (player *Player) PlayUri(uri string) {
	log.Println("Start mpv with ", uri)
	player.mpv = exec.Command("mpv", "--no-video", uri)
	go player.runMpv()
}

func (player *Player) PlayData(data io.Reader) {
	if player.mpv != nil {
		log.Fatal("Player has already an mpv subprocess")
	}

	player.mpv = exec.Command("mpv", "-")
	stdin, err := player.mpv.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		defer stdin.Close()

		if _, err := io.Copy(stdin, data); err != nil {
			log.Fatal(fmt.Sprintf("Failed to write song data to mpv (%v)", err))
		}
	}()

	go player.runMpv()
}

func (player *Player) Pause() {
	if player.mpv != nil {
		log.Println("Send SIGSTOP to mpv subprocess")
		player.mpv.Process.Signal(syscall.SIGSTOP)
	}
}

func (player *Player) Search(pattern string) []Song {
	results := []Song{}
	for _, backend := range player.Backends {
		results = append(results, backend.Search(pattern)...)
	}
	return results
}
