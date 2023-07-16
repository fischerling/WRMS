package main

// The spotify backend is based on the librespot-golang code released under MIT
// license.
// Authors of the librespot-golang code are:
// Copyright (c) 2015 Paul Lietar
// Copyright (c) 2015 badfortrains
// Copyright (c) 2018 Guillaume "xplodwild" Lesniak

import (
	"errors"
	"os"
	"strings"

	"github.com/librespot-org/librespot-golang/Spotify"
	"github.com/librespot-org/librespot-golang/librespot"
	"github.com/librespot-org/librespot-golang/librespot/core"
	"github.com/librespot-org/librespot-golang/librespot/utils"

	"muhq.space/go/wrms/llog"
)

const (
	deviceName = "wrms"
)

type SpotifyConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type SpotifyBackend struct {
	session *core.Session
}

func NewSpotify(config *SpotifyConfig) (*SpotifyBackend, error) {
	spotify := SpotifyBackend{}
	if config == nil {
		config = &SpotifyConfig{}
	}

	usernameEnv := os.Getenv("WRMS_SPOTIFY_USER")
	if usernameEnv != "" {
		config.Username = usernameEnv
	}

	passwordEnv := os.Getenv("WRMS_SPOTIFY_PASSWORD")
	if passwordEnv != "" {
		config.Password = passwordEnv
	}

	if config.Username == "" || config.Password == "" {
		return nil, errors.New("User and password for spotify must be provided")
	}

	var err error
	spotify.session, err = librespot.Login(config.Username, config.Password, deviceName)
	if err != nil {
		return nil, err
	}

	return &spotify, nil
}

func (_ *SpotifyBackend) OnSongFinished(*Song) {}

func (spotify *SpotifyBackend) Play(song *Song, player Player) {
	trackID := song.Uri
	session := spotify.session
	llog.Debug("Loading track for play: %v", trackID)

	// Get the track metadata: it holds information about which files and encodings are available
	track, err := session.Mercury().GetTrack(utils.Base62ToHex(trackID))
	if err != nil {
		llog.Error("Error loading track: %s", err)
		return
	}

	// For now, select the OGG 160kbps variant of the track. The "high quality"
	// setting in the official Spotify app is the OGG 320kbps variant.
	var selectedFile *Spotify.AudioFile
	llog.DDebug("Available Formats:")
	for _, file := range track.GetFile() {
		llog.DDebug("- %v", file.GetFormat())
		if file.GetFormat() == Spotify.AudioFile_OGG_VORBIS_160 {
			selectedFile = file
		}
	}

	// Synchronously load the track
	audioFile, err := session.Player().LoadTrack(selectedFile, track.GetGid())
	if err != nil {
		llog.Fatal("Error while loading track: %s\n", err)
	}

	player.PlayData(audioFile)
}

func (spotify *SpotifyBackend) Search(patterns map[string]string) []Song {
	session := spotify.session
	keyword := ""
	for _, p := range []string{"pattern", "title", "artist", "album"} {
		if v, ok := patterns[p]; ok {
			keyword = v
			break
		}
	}

	resp, err := session.Mercury().Search(keyword, 12, session.Country(), session.Username())

	if err != nil {
		llog.Error("Failed to search: %s", err)
		return []Song{}
	}

	res := resp.Results

	llog.DDebug("Search results for %s", keyword)
	llog.DDebug("=============================")

	if res.Error != nil {
		llog.Error("Search result error: %s", res.Error)
	}

	llog.Debug("\nTracks: %d (total %d)\n", len(res.Tracks.Hits), res.Tracks.Total)

	results := []Song{}
	for _, track := range res.Tracks.Hits {
		uriParts := strings.Split(track.Uri, ":")
		s := NewSong(track.Name, track.Artists[0].Name, "spotify", uriParts[len(uriParts)-1])
		s.Album = track.Album.Name
		results = append(results, s)
	}

	return results
}
