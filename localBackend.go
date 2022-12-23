package main

import (
	"fmt"
	"github.com/dhowden/tag"
	"muhq.space/go/wrms/llog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

type LocalBackend struct {
	musicDir string
	lock     sync.RWMutex
	songs    map[string]*Song
}

func NewLocalBackend(musicDir string) *LocalBackend {
	b := LocalBackend{musicDir, sync.RWMutex{}, map[string]*Song{}}

	go b.findSongs()
	return &b
}

func (b *LocalBackend) insert(song *Song) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.songs[song.Uri] = song
}

func (b *LocalBackend) findSongs() {
	llog.Debug(fmt.Sprintf("Starting song search under: %s", b.musicDir))

	// Insert some Songs
	song := NewSong("No!", "Bukahara", "local", "Bukahara/Phantasma/01-No!.ogg")

	for _, s := range []*Song{&song} {
		b.insert(s)
	}

	err := filepath.Walk(b.musicDir, func(p string, finfo os.FileInfo, err error) error {
		if err != nil {
			llog.Warning(fmt.Sprintf("error %v at path %s\n", err, p))
			return err
		}

		if !finfo.Mode().IsRegular() {
			return nil
		}

		ext := strings.ToLower(path.Ext(p))
		for _, excluded := range []string{".png", ".jpg", ".txt", ".pdf", ".m3u"} {
			if ext == excluded {
				return nil
			}
		}

		f, err := os.Open(p)
		defer f.Close()
		if err != nil {
			llog.Warning(fmt.Sprintf("error %v opening file %s", err, p))
			return err
		}

		m, err := tag.ReadFrom(f)
		if err != nil {
			llog.Warning(fmt.Sprintf("error reading tags from %s: %v", p, err))
			// Contiune walking
			return nil
		}

		s := NewSong(m.Title(), m.Artist(), "local", p)
		b.insert(&s)
		return nil
	})

	if err != nil {
		llog.Error(fmt.Sprintf("error walking the path %q: %v", b.musicDir, err))
	}
}

func (b *LocalBackend) Play(song *Song, player *Player) {
	player.PlayUri("file://" + song.Uri)
}

func (b *LocalBackend) Search(keyword string) []Song {
	pattern := strings.ToLower(keyword)
	results := []Song{}

	b.lock.RLock()
	defer b.lock.RUnlock()

	for _, s := range b.songs {
		if strings.Contains(strings.ToLower(s.Title), pattern) ||
			strings.Contains(strings.ToLower(s.Artist), pattern) {
			results = append(results, *s)
		}
	}

	return results
}
