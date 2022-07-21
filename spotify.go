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

	// "github.com/librespot-org/librespot-golang/Spotify"
	"github.com/librespot-org/librespot-golang/librespot"
	"github.com/librespot-org/librespot-golang/librespot/core"
	// "github.com/librespot-org/librespot-golang/librespot/utils"
	"github.com/xlab/portaudio-go/portaudio"
	// "github.com/xlab/vorbis-go/decoder"
)

const (
	deviceName = "wrms"
	// The number of samples per channel in the decoded audio
	samplesPerChannel = 2048
	// The samples bit depth
	bitDepth = 16
	// The samples format
	sampleFormat = portaudio.PaFloat32
)

type SpotifyBackend struct {
	session *core.Session
}

func NewSpotify() (*SpotifyBackend, error) {
	spotify := SpotifyBackend{}

	if err := portaudio.Initialize(); paError(err) {
		return nil, errors.New(paErrorText(err))
	}

	var err error

	username := os.Getenv("WRMS_SPOTIFY_USER")
	password := os.Getenv("WRMS_SPOTIFY_PASSWORD")
	if username == "" || password == "" {
		return nil, errors.New("User and password for spotify must be provided")
	}

	spotify.session, err = librespot.Login(username, password, deviceName)

	if err != nil {
		return nil, err
	}

	return &spotify, nil
}

func (spotify *SpotifyBackend) Play(song Song) {
}

func (spotify *SpotifyBackend) Pause() {
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

	for _, track := range res.Tracks.Hits {
		fmt.Printf(" => %v (%s)\n", track, track.Uri)
	}

	return []Song{}
}

// PortAudio helpers
func paError(err portaudio.Error) bool {
	return portaudio.ErrorCode(err) != portaudio.PaNoError
}

func paErrorText(err portaudio.Error) string {
	return "PortAudio error: " + portaudio.GetErrorText(err)
}
