package main

import (
	"io"
	"os/exec"
	"sync"
	"syscall"

	"muhq.space/go/wrms/llog"
)

type Player struct {
	Backends map[string]Backend
	mpv      *exec.Cmd
	wrms     *Wrms
}

func NewPlayer(wrms *Wrms, backends []string) Player {
	availableBackends := map[string]Backend{}

	var b Backend
	var err error
	for _, backend := range backends {
		switch backend {
		case "spotify":
			b, err = NewSpotify(wrms.Config.spotify)
		case "youtube":
			b = NewYoutubeBackend()
		case "dummy":
			b = &DummyBackend{}
		case "local":
			b = NewLocalBackend(wrms.Config.LocalMusicDir)
		case "upload":
			b = NewUploadBackend(wrms.Config.UploadDir)
		default:
			llog.Error("Not supported backend %s", backend)
		}

		if err != nil {
			llog.Error("Error during initialization of the %s backend: %s", backend, err.Error())
		} else {
			availableBackends[backend] = b
		}
	}

	return Player{availableBackends, nil, wrms}
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

	wrms := player.wrms

	llog.Info("mpv finished. Resetting mpv, the current song and call play")
	player.mpv = nil
	wrms.Playing = false

	player.Backends[wrms.CurrentSong.Source].OnSongFinished(&wrms.CurrentSong)
	wrms.CurrentSong.Uri = ""
	wrms.PlayPause()
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

func (player *Player) Search(pattern string) chan []Song {
	var wg sync.WaitGroup
	wg.Add(len(player.Backends))

	ch := make(chan []Song)

	for _, backend := range player.Backends {
		backend := backend
		go func() {
			ch <- backend.Search(pattern)
			wg.Done()
		}()
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	return ch
}
