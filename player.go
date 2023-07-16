package main

import (
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"muhq.space/go/wrms/llog"
)

type Player interface {
	Play(*Song)
	Playing() bool
	Search(map[string]string) chan []Song
	PlayUri(string)
	PlayData(io.Reader)
	Pause()
	Continue()
	Stop()
}

// command struct used to serialize player commands
type cmd struct {
	cmd  string
	data io.Reader
	uri  string
}

type MpvPlayer struct {
	Backends map[string]Backend
	wrms     *Wrms
	mpv      atomic.Pointer[exec.Cmd]
	cmdQueue chan cmd
}

func NewMpvPlayer(wrms *Wrms, backends []string) *MpvPlayer {
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

	p := &MpvPlayer{
		Backends: availableBackends,
		wrms:     wrms,
		cmdQueue: make(chan cmd)}
	go p.serveCmds()
	return p
}

const MPV_FLAGS = "--no-video"

func mpvArgv(uri string) []string {
	cmd := []string{uri}
	cmd = append(cmd, strings.Split(MPV_FLAGS, " ")...)
	if wrms.Config.MpvFlags != "" {
		cmd = append(cmd, strings.Split(wrms.Config.MpvFlags, " ")...)
	}
	return cmd
}

func (player *MpvPlayer) startMpv(uri string) *exec.Cmd {
	if player.mpv.Load() != nil {
		llog.Fatal("Player has already an mpv subprocess")
	}
	llog.Info("Start mpv to play %s", uri)

	cmd := mpvArgv(uri)
	llog.Debug("Running 'mpv %s'", strings.Join(cmd, " "))

	// Since all player controll are serialized through cmdQueue no one
	// else can modify player.mpv at this point.
	mpv := exec.Command("mpv", cmd...)
	player.mpv.Store(mpv)
	return mpv
}

func (player *MpvPlayer) runMpv() {
	// Remember the song we are playing
	// TODO: This is potentially racy
	currentSong := wrms.CurrentSong.Load()

	mpv := player.mpv.Load()
	output, err := mpv.CombinedOutput()
	if err != nil {
		// mpv returns exit code 4 if it teminates due to a signal
		if err.(*exec.ExitError).ExitCode() != 4 {
			llog.Debug("Mpv output: %s", output)
			llog.Fatal("Mpv failed with: %s", err)
		}
	}

	player.mpv.Store(nil)
	player.Backends[currentSong.Source].OnSongFinished(currentSong)

	// mpv terminated because it finished playing the song
	if err == nil {
		llog.Info("mpv finished. Resetting mpv, and calling next")
		wrms._lockedNext()
	}
}

func (p *MpvPlayer) serveCmds() {
	for cmd := range p.cmdQueue {
		switch cmd.cmd {
		case "playData":
			p._playData(cmd.data)
		case "playUri":
			p._playUri(cmd.uri)
		case "pause":
			p._pause()
		case "continue":
			p._continue()
		case "stop":
			p._stop()
		}
	}
}

// Play arbitrary media using mpv
func (p *MpvPlayer) PlayUri(uri string)      { p.cmdQueue <- cmd{cmd: "playUri", uri: uri} }
func (p *MpvPlayer) PlayData(data io.Reader) { p.cmdQueue <- cmd{cmd: "playData", data: data} }

// Controls
func (p *MpvPlayer) Pause()    { p.cmdQueue <- cmd{cmd: "pause"} }
func (p *MpvPlayer) Continue() { p.cmdQueue <- cmd{cmd: "continue"} }
func (p *MpvPlayer) Stop()     { p.cmdQueue <- cmd{cmd: "stop"} }
func (p *MpvPlayer) Close()    { close(p.cmdQueue) }

// Double dispatch play entry point
func (p *MpvPlayer) Play(song *Song) {
	llog.Info("Start playing %v", song)
	p.Backends[song.Source].Play(song, p)
}

func (p *MpvPlayer) Playing() bool {
	return p.mpv.Load() != nil
}

func (player *MpvPlayer) _playUri(uri string) {
	player.startMpv(uri)
	go player.runMpv()
}

func (player *MpvPlayer) _playData(data io.Reader) {
	mpv := player.startMpv("-")

	stdin, err := mpv.StdinPipe()
	if err != nil {
		llog.Fatal("Connecting to mpv Pipe failed: %v", err)
	}

	go func() {
		defer stdin.Close()

		if _, err := io.Copy(stdin, data); err != nil {
			llog.Warning("Failed to write song data to mpv: %v", err)
		}
	}()

	go player.runMpv()
}

func (player *MpvPlayer) _pause() {
	mpv := player.mpv.Load()
	if mpv == nil {
		llog.Warning("No mpv process to pause")
		return
	}

	llog.Debug("Send SIGSTOP to mpv subprocess")
	err := mpv.Process.Signal(syscall.SIGSTOP)
	if err != nil {
		llog.Fatal("Failed to send SIGSTOP to mpv")
	}
}

func (player *MpvPlayer) _continue() {
	mpv := player.mpv.Load()
	if mpv == nil {
		llog.Warning("Continue Play but there is no running mpv process to continue")
		return
	}

	llog.Debug("Send SIGCONT to mpv subprocess")
	err := mpv.Process.Signal(syscall.SIGCONT)
	if err != nil {
		llog.Fatal("Failed to send SIGCONT to mpv")
	}
}

func (player *MpvPlayer) _stop() {
	mpv := player.mpv.Load()
	if mpv == nil {
		// Wrms.Next() may race with Player.runMpv() therefore this must not be a hard error
		llog.Warning("There is no mpv process to terminate")
		return
	}

	// mpv is actually running.
	if mpv.Process != nil {
		llog.Debug("Send SIGTERM to mpv subprocess")
		mpv.Process.Signal(syscall.SIGTERM)
	} else {
		llog.Fatal("mpv process is not running yet")
	}

	player.mpv.Store(nil)
}

func (player *MpvPlayer) Search(pattern map[string]string) chan []Song {
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
