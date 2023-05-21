package main

import (
	"io"
	"os/exec"
	"syscall"

	"muhq.space/go/wrms/llog"
)

type Player struct {
	Backends map[string]Backend
	mpv      *exec.Cmd
	wrms     *Wrms
}

func NewPlayer(wrms *Wrms, backends []string) Player {
	available_backends := map[string]Backend{}

	var b Backend
	var err error
	for _, backend := range backends {
		switch backend {
		case "spotify":
			b, err = NewSpotify()
		case "youtube":
			b = NewYoutubeBackend()
		case "dummy":
			b = &DummyBackend{}
		case "local":
			b = NewLocalBackend(wrms.Config.localMusicDir)
		default:
			llog.Error("Not supported backend %s", backend)
		}

		if err != nil {
			llog.Error("Error during initialization of the %s backend: %s", backend, err.Error())
		} else {
			available_backends[backend] = b
		}
	}

	return Player{available_backends, nil, wrms}
}

func (player *Player) Play(song *Song) {
	if player.mpv != nil {
		llog.Debug("Send SIGCONT to mpv subprocess")
		err := player.mpv.Process.Signal(syscall.SIGCONT)
		if err != nil {
			llog.Fatal("Failed to send SIGCONT to mpv")
		}
		return
	}

	llog.Info("Start playing %v", song)
	player.Backends[song.Source].Play(song, player)
}

func (player *Player) runMpv() {
	_, err := player.mpv.CombinedOutput()
	if err != nil {
		llog.Fatal(err.Error())
	}

	llog.Info("mpv finished. Resetting mpv, the current song and call play")
	player.mpv = nil
	player.wrms.Playing = false
	player.wrms.CurrentSong.Uri = ""
	player.wrms.PlayPause()
}

func (player *Player) PlayUri(uri string) {
	llog.Info("Start mpv with %s", uri)
	player.mpv = exec.Command("mpv", "--no-video", uri)
	go player.runMpv()
}

func (player *Player) PlayData(data io.Reader) {
	if player.mpv != nil {
		llog.Fatal("Player has already an mpv subprocess")
	}

	player.mpv = exec.Command("mpv", "-")
	stdin, err := player.mpv.StdinPipe()
	if err != nil {
		llog.Fatal(err.Error())
	}

	go func() {
		defer stdin.Close()

		if _, err := io.Copy(stdin, data); err != nil {
			llog.Fatal("Failed to write song data to mpv (%v)", err)
		}
	}()

	go player.runMpv()
}

func (player *Player) Pause() {
	if player.mpv != nil {
		llog.Debug("Send SIGSTOP to mpv subprocess")
		err := player.mpv.Process.Signal(syscall.SIGSTOP)
		if err != nil {
			llog.Fatal("Failed to send SIGSTOP to mpv")
		}
	}
}

func (player *Player) Search(pattern string) []Song {
	results := []Song{}
	for _, backend := range player.Backends {
		results = append(results, backend.Search(pattern)...)
	}
	return results
}
