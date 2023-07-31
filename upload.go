package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path"
	// "path/filepath"
	"io"

	"github.com/dhowden/tag"
	"muhq.space/go/wrms/llog"
)

type UploadBackend struct {
	uploadDir string
}

func NewUploadBackend(uploadDir string) (*UploadBackend, error) {
	b := UploadBackend{uploadDir: uploadDir}

	dirInfo, err := os.Stat(uploadDir)

	if os.IsNotExist(err) {
		if err = os.MkdirAll(uploadDir, 0750); err != nil {
			return nil, err
		}
	} else if !dirInfo.IsDir() {
		llog.Fatal("upload directory %s exists and is not a directory", uploadDir)
	}

	b.setupUploadRoute()

	return &b, nil
}

func (b *UploadBackend) upload(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		llog.Warning("Failed to read request body: %s", string(data))
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	fileName := r.URL.Query().Get("song")

	filePath := path.Join(b.uploadDir, fileName)
	f, err := os.Create(filePath)
	if err != nil {
		llog.Fatal("Failed to create uploaded %s song on disk: %s", filePath, err)
	}

	_, err = f.Write(data)
	if err != nil {
		llog.Fatal("Failed to write uploaded song on disk: %s", err)
	}

	f.Close()

	var title string
	var artist string

	dataReader := bytes.NewReader(data)
	m, err := tag.ReadFrom(dataReader)

	if err != nil {
		llog.Warning("error reading tags from %s: %v", fileName, err)
	} else {
		title = m.Title()
		artist = m.Artist()
	}

	if title == "" {
		title = fileName
	}

	if artist == "" {
		artist = "Unknown"
	}

	wrms.AddSong(NewDetailedSong(title, artist, "upload", fileName, m.Album(), m.Year()))
	fmt.Fprintf(w, "Added uploaded song %s", string(fileName))
}

func (b *UploadBackend) setupUploadRoute() {
	http.HandleFunc("/upload", b.upload)
}

func (b *UploadBackend) OnSongFinished(song *Song) {
	songPath := path.Join(b.uploadDir, song.Uri)
	llog.Debug("Removing finished song %s", songPath)
	os.Remove(songPath)
}

func (b *UploadBackend) Play(song *Song, player Player) {
	player.PlayUri("file://" + path.Join(b.uploadDir, song.Uri))
}

func (b *UploadBackend) Search(map[string]string) []*Song {
	return nil
}
