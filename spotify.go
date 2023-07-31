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
	"regexp"
	"strings"

	"github.com/fischerling/librespot-golang/Spotify"
	"github.com/fischerling/librespot-golang/librespot"
	"github.com/fischerling/librespot-golang/librespot/core"
	"github.com/fischerling/librespot-golang/librespot/utils"

	"muhq.space/go/wrms/llog"
)

const (
	deviceName             = "wrms"
	searchResults          = 25
	displayedSearchResults = 10
)

type SpotifyConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type SpotifyBackend struct {
	session                *core.Session
	searchResults          int
	displayedSearchResults int
}

func NewSpotify(config *SpotifyConfig) (*SpotifyBackend, error) {
	spotify := SpotifyBackend{
		searchResults:          searchResults,
		displayedSearchResults: displayedSearchResults}

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

func (spotify *SpotifyBackend) Search(patterns map[string]string) []*Song {
	session := spotify.session
	results := []*Song{}
	resultMap := make(map[string]struct{})

	for _, p := range []string{"pattern", "title", "artist", "album"} {
		pattern, ok := patterns[p]
		if !ok {
			continue
		}

		resp, err := session.Mercury().Search(pattern,
			spotify.searchResults,
			session.Country(),
			session.Username())

		if err != nil {
			llog.Error("Failed to search: %s", err)
			return nil
		}

		res := resp.Results
		llog.DDebug("Search results for %v", pattern)
		llog.DDebug("=============================")

		if res.Error != nil {
			llog.Error("Search result error: %s", res.Error)
		}
		llog.DDebug("\nTracks: %d (total %d)\n", len(res.Tracks.Hits), res.Tracks.Total)

	out:
		for _, track := range res.Tracks.Hits {
			// filter matching tracks
			for _, cond := range []string{"pattern", "title", "artist", "album"} {
				cond_pattern, ok := patterns[cond]
				if !ok {
					continue
				}

				simpleMatch, err := regexp.Compile(fmt.Sprintf(".*%s.*", cond_pattern))
				if err != nil {
					llog.Fatal("Failed to compile matching regex '%s': %v",
						fmt.Sprintf(".*%s.*", cond_pattern), err)
				}

				switch cond {
				case "title":
					if !simpleMatch.MatchString(track.Name) {
						continue out
					}

				case "album":
					if !simpleMatch.MatchString(track.Album.Name) {
						continue out
					}

				case "artist":
					matchingArtist := false
					for _, a := range track.Artists {
						if simpleMatch.MatchString(a.Name) {
							matchingArtist = true
							break
						}
					}

					if !matchingArtist {
						continue out
					}
				}
			}

			uriParts := strings.Split(track.Uri, ":")
			uri := uriParts[len(uriParts)-1]

			if _, ok := resultMap[uri]; !ok {
				s := NewSong(track.Name, track.Artists[0].Name, "spotify", uri)
				s.Album = track.Album.Name
				resultMap[uri] = struct{}{}
				results = append(results, s)
			}
		}
	}

	// Think about a way to better sort ther results by relevance
	if len(results) > spotify.displayedSearchResults {
		return results[:spotify.displayedSearchResults]
	}
	return results
}

func (spotify *SpotifyBackend) loadPlaylist(playlist string) []*Song {
	playlistIdRegex := regexp.MustCompile(`playlist/.*\?`)
	match := playlistIdRegex.FindString(playlist)
	if match == "" {
		llog.Error("Failed to extract spotify playlist id from %s", playlist)
	}

	id := match[len("playlist/") : len(match)-1]
	llog.Debug("Extracted id %s from spotify playlist URI %s", id, playlist)

	// _pl, err := GetPlaylistV2(spotify.session.Mercury(), id)
	_pl, err := spotify.session.Mercury().GetPlaylist(id)
	if err != nil {
		llog.Error("Getting playlist %s failed with %v", playlist, err)
		return nil
	}

	plContent := _pl.GetContents()
	if plContent == nil {
		llog.Warning("Playlist %s has no content", playlist)
		return nil
	}

	plItems := plContent.GetItems()
	if plItems == nil {
		llog.Warning("Playlist %s content has no items", playlist)
		return nil
	}

	var songs []*Song
	for _, item := range plItems {
		uri := item.GetUri()
		if uri == "" {
			continue
		}

		uriParts := strings.Split(uri, ":")
		uri = uriParts[len(uriParts)-1]
		track, err := spotify.session.Mercury().GetTrack(utils.Base62ToHex(uri))
		if err != nil {
			llog.Error("Failed to load track %s: %v", uri, err)
		}

		s := NewSong(*track.Name, *track.Artist[0].Name, "spotify", uri)
		s.Album = *track.Album.Name
		songs = append(songs, s)
	}

	llog.Debug("Added %d songs from spotify playlist %s", len(songs), _pl.Attributes.GetName())
	return songs
}
