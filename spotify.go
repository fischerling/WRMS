package main

// The spotify backend is based on the librespot-golang code released under MIT
// license.
// Authors of the librespot-golang code are:
// Copyright (c) 2015 Paul Lietar
// Copyright (c) 2015 badfortrains
// Copyright (c) 2018 Guillaume "xplodwild" Lesniak

import (
	"errors"
	"fmt"
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

func (spotify *SpotifyBackend) Play(song *Song, player *Player) {
	trackID := song.Uri
	session := spotify.session
	fmt.Println("Loading track for play: ", trackID)

	// Get the track metadata: it holds information about which files and encodings are available
	track, err := session.Mercury().GetTrack(utils.Base62ToHex(trackID))
	if err != nil {
		fmt.Println("Error loading track: ", err)
		return
	}

	fmt.Println("Track:", track.GetName())

	// For now, select the OGG 160kbps variant of the track. The "high quality"
	// setting in the official Spotify app is the OGG 320kbps variant.
	var selectedFile *Spotify.AudioFile
	for _, file := range track.GetFile() {
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

func (spotify *SpotifyBackend) Search(keyword string) []Song {
	session := spotify.session
	resp, err := session.Mercury().Search(keyword, 12, session.Country(), session.Username())

	if err != nil {
		fmt.Println("Failed to search:", err)
		return []Song{}
	}

	res := resp.Results

	fmt.Println("Search results for ", keyword)
	fmt.Println("=============================")

	if res.Error != nil {
		fmt.Println("Search result error:", res.Error)
	}

	fmt.Printf("\nTracks: %d (total %d)\n", len(res.Tracks.Hits), res.Tracks.Total)

	results := []Song{}
	for _, track := range res.Tracks.Hits {
		uriParts := strings.Split(track.Uri, ":")
		results = append(results, NewSong(track.Name, track.Artists[0].Name, "spotify", uriParts[len(uriParts)-1]))
		// fmt.Printf(" => %v (%s)\n", track, track.Uri)
	}

	return results
}
