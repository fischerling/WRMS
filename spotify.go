package main

// The spotify backend is based on the librespot-golang code released under MIT
// license.
// Authors of the librespot-golang code are:
// Copyright (c) 2015 Paul Lietar
// Copyright (c) 2015 badfortrains
// Copyright (c) 2018 Guillaume "xplodwild" Lesniak

import (
	"errors"
	"log"
	"fmt"
	"os"
	"strings"
	"sync"
	"unsafe"

	"github.com/librespot-org/librespot-golang/Spotify"
	"github.com/librespot-org/librespot-golang/librespot"
	"github.com/librespot-org/librespot-golang/librespot/core"
	"github.com/librespot-org/librespot-golang/librespot/utils"
	"github.com/xlab/portaudio-go/portaudio"
	"github.com/xlab/vorbis-go/decoder"
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

func (spotify *SpotifyBackend) Play(song *Song) {
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

	// As a demo, select the OGG 160kbps variant of the track. The "high quality" setting in the official Spotify
	// app is the OGG 320kbps variant.
	var selectedFile *Spotify.AudioFile
	for _, file := range track.GetFile() {
		if file.GetFormat() == Spotify.AudioFile_OGG_VORBIS_160 {
			selectedFile = file
		}
	}

	// Synchronously load the track
	audioFile, err := session.Player().LoadTrack(selectedFile, track.GetGid())

	// TODO: channel to be notified of chunks downloaded (or reader?)

	if err != nil {
		fmt.Printf("Error while loading track: %s\n", err)
	} else {
		// We have the track audio, let's play it! Initialize the OGG decoder, and start a PortAudio stream.
		// Note that we skip the first 167 bytes as it is a Spotify-specific header. You can decode it by
		// using this: https://sourceforge.net/p/despotify/code/HEAD/tree/java/trunk/src/main/java/se/despotify/client/player/SpotifyOggHeader.java
		fmt.Println("Setting up OGG decoder...")
		dec, err := decoder.New(audioFile, samplesPerChannel)
		if err != nil {
			log.Fatalln(err)
		}

		info := dec.Info()

		go func() {
			dec.Decode()
			dec.Close()
		}()

		fmt.Println("Setting up PortAudio stream...")
		fmt.Printf("PortAudio channels: %d / SampleRate: %f\n", info.Channels, info.SampleRate)

		var wg sync.WaitGroup
		var stream *portaudio.Stream
		callback := paCallback(&wg, int(info.Channels), dec.SamplesOut())

		if err := portaudio.OpenDefaultStream(&stream, 0, info.Channels, sampleFormat, info.SampleRate,
			samplesPerChannel, callback, nil); paError(err) {
			log.Fatalln(paErrorText(err))
		}

		fmt.Println("Starting playback...")
		if err := portaudio.StartStream(stream); paError(err) {
			log.Fatalln(paErrorText(err))
		}

		wg.Wait()
	}
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

	results := []Song{}
	for _, track := range res.Tracks.Hits {
		uriParts := strings.Split(track.Uri, ":")
		results = append(results, NewSong(track.Name, track.Artists[0].Name, "spotify", uriParts[len(uriParts) - 1]))
		// fmt.Printf(" => %v (%s)\n", track, track.Uri)
	}

	return results
}

// PortAudio helpers
func paError(err portaudio.Error) bool {
	return portaudio.ErrorCode(err) != portaudio.PaNoError
}

func paErrorText(err portaudio.Error) string {
	return "PortAudio error: " + portaudio.GetErrorText(err)
}

func paCallback(wg *sync.WaitGroup, channels int, samples <-chan [][]float32) portaudio.StreamCallback {
	wg.Add(1)
	return func(_ unsafe.Pointer, output unsafe.Pointer, sampleCount uint,
		_ *portaudio.StreamCallbackTimeInfo, _ portaudio.StreamCallbackFlags, _ unsafe.Pointer) int32 {

		const (
			statusContinue = int32(portaudio.PaContinue)
			statusComplete = int32(portaudio.PaComplete)
		)

		frame, ok := <-samples
		if !ok {
			wg.Done()
			return statusComplete
		}
		if len(frame) > int(sampleCount) {
			frame = frame[:sampleCount]
		}

		var idx int
		out := (*(*[1 << 32]float32)(unsafe.Pointer(output)))[:int(sampleCount)*channels]
		for _, sample := range frame {
			if len(sample) > channels {
				sample = sample[:channels]
			}
			for i := range sample {
				out[idx] = sample[i]
				idx++
			}
		}

		return statusContinue
	}
}
