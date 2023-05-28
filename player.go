package main

import (
	"io"
	"os/exec"
	"strings"
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
			b, err = NewSpotify(wrms.Config.Spotify)
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
	output, err := player.mpv.CombinedOutput()
	if err != nil {
		llog.Debug("Mpv output: %s", output)
		llog.Fatal("Mpv failed with: %s", err.Error())
	}

	wrms := player.wrms

	llog.Info("mpv finished. Resetting mpv, and calling next")
	player.mpv = nil

	player.Backends[wrms.CurrentSong.Source].OnSongFinished(&wrms.CurrentSong)
	wrms.Next()
}

const MPV_FLAGS = "--no-video"

func mpvArgv(uri string) []string {
	cmd := []string{"mpv", uri}
	cmd = append(cmd, strings.Split(MPV_FLAGS, " ")...)
	cmd = append(cmd, strings.Split(wrms.Config.MpvFlags, " ")...)
	return cmd
}

func (player *Player) startMpv(uri string) {
	if player.mpv != nil {
		llog.Fatal("Player has already an mpv subprocess")
	}
	llog.Info("Start mpv to play %s", uri)

	cmd := mpvArgv(uri)
	llog.Debug("Running '%s'", strings.Join(cmd, " "))
	player.mpv = exec.Command("mpv", cmd...)
}

func (player *Player) PlayUri(uri string) {
	player.startMpv(uri)
	go player.runMpv()
}

func (player *Player) PlayData(data io.Reader) {
	player.startMpv("-")

	stdin, err := player.mpv.StdinPipe()
	if err != nil {
		llog.Fatal("Connecting to mpv Pipe failed: %v", err)
	}

	go func() {
		defer stdin.Close()

		if _, err := io.Copy(stdin, data); err != nil {
			llog.Fatal("Failed to write song data to mpv: %v", err)
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
