package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path"
	// "path/filepath"
	"io/ioutil"

	"github.com/dhowden/tag"
	"muhq.space/go/wrms/llog"
)

type UploadBackend struct {
	uploadDir string
}

func NewUploadBackend(uploadDir string) *UploadBackend {
	b := UploadBackend{uploadDir: uploadDir}

	dirInfo, err := os.Stat(uploadDir)

	if os.IsNotExist(err) {
		os.Mkdir(uploadDir, 0750)
	} else if !dirInfo.IsDir() {
		llog.Fatal("upload directory %s exists and is not a directory", uploadDir)
	}

	b.setupUploadRoute()

	return &b
}

func (b *UploadBackend) upload(w http.ResponseWriter, r *http.Request) {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		llog.Warning("Failed to read request body: %s", string(data))
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	fileName := r.URL.Query().Get("song")

	f, err := os.Create(path.Join(b.uploadDir, fileName))
	if err != nil {
		llog.Fatal("Failed to create uploaded song on disk")
	}

	_, err = f.Write(data)
	if err != nil {
		llog.Fatal("Failed to write uploaded song on disk")
	}

	f.Close()

	var song Song
	dataReader := bytes.NewReader(data)
	m, err := tag.ReadFrom(dataReader)
	if err != nil {
		llog.Warning("error reading tags from %s: %v", fileName, err)
		song = NewSong(fileName, "Unknown", "upload", fileName)
	} else {
		song = NewSong(m.Title(), m.Artist(), "upload", fileName)
	}

	wrms.AddSong(song)
	wrms.Broadcast(Event{"add", []Song{song}})
	fmt.Fprintf(w, "Added uploaded song %s", string(fileName))
}

func (b *UploadBackend) setupUploadRoute() {
	http.HandleFunc("/upload", b.upload)
}

func (b *UploadBackend) Play(song *Song, player *Player) {
	player.PlayUri("file://" + path.Join(b.uploadDir, song.Uri))
}

func (b *UploadBackend) Search(keyword string) []Song {
	return nil
}
