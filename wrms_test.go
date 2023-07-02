package main

import (
	"github.com/google/uuid"
	"io"
	"muhq.space/go/wrms/llog"
	"testing"
)

type mockPlayer struct{}

func (p *mockPlayer) Play(*Song)                              {}
func (p *mockPlayer) PlayUri(string)                          {}
func (p *mockPlayer) PlayData(io.Reader)                      {}
func (p *mockPlayer) Search(pattern string) (res chan []Song) { return }
func (p *mockPlayer) Playing() bool                           { return false }
func (p *mockPlayer) Pause()                                  {}
func (p *mockPlayer) Continue()                               {}
func (p *mockPlayer) Stop()                                   {}

var alice, _ = uuid.NewRandom()

func TestWrmsAdd(t *testing.T) {
	wrms := Wrms{Player: &mockPlayer{}}
	s1 := NewDummySong("song1", "snfmt")
	wrms.AddSong(s1)

	if len(wrms.queue) != 1 {
		t.Log("len should be 1")
		t.Fail()
	}
}

func TestWrmsAdjust(t *testing.T) {
	wrms := Wrms{Player: &mockPlayer{}}
	s1 := NewDummySong("song1", "snfmt")
	wrms.AddSong(s1)

	wrms.AdjustSongWeight(alice, s1.Uri, "up")
	if wrms.queue[0].Weight != 1 {
		t.Log("song weight should be 1")
		t.Fail()
	}
}

func TestWrmsUpDownNext(t *testing.T) {
	wrms := Wrms{Player: &mockPlayer{}}
	s1 := NewDummySong("song1", "snfmt")
	s2 := NewDummySong("song2", "snfmt")
	wrms.AddSong(s1)
	wrms.AddSong(s2)

	wrms.AdjustSongWeight(alice, s1.Uri, "down")
	wrms.AdjustSongWeight(alice, s2.Uri, "up")
	wrms.Next()
	if wrms.CurrentSong.Load().Uri != s2.Uri {
		t.Log("Not retrning the upvoted song s2")
		t.Fail()
	}
}

func TestWrmsUpUpDownNextNext(t *testing.T) {
	wrms := Wrms{Player: &mockPlayer{}}
	s1 := NewDummySong("song1", "snfmt")
	s2 := NewDummySong("song2", "snfmt")
	s3 := NewDummySong("song3", "snfmt")
	wrms.AddSong(s1)
	wrms.AddSong(s2)
	wrms.AddSong(s3)

	llog.Info("UpUpDown test")
	wrms.AdjustSongWeight(alice, s1.Uri, "up")
	wrms.AdjustSongWeight(alice, s2.Uri, "down")
	wrms.AdjustSongWeight(alice, s3.Uri, "up")
	wrms.Next()
	if wrms.CurrentSong.Load().Uri == s2.Uri {
		t.Log("Playing the downvoted song s2")
		t.Fail()
	}
	wrms.Next()
	if wrms.CurrentSong.Load().Uri == s2.Uri {
		t.Log("Playing the downvoted song s2")
		t.Fail()
	}
}

func TestWrmsDoubleAdd(t *testing.T) {
	wrms := Wrms{Player: &mockPlayer{}}
	s1 := NewDummySong("song1", "snfmt")

	llog.Info("Double Add test")
	wrms.AddSong(s1)
	wrms.AdjustSongWeight(alice, s1.Uri, "up")
	wrms.Next()

	s1 = NewDummySong("song1", "snfmt")
	wrms.AddSong(s1)
	wrms.AdjustSongWeight(alice, s1.Uri, "up")
	wrms.Next()
}
