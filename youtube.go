package main

import (
	"encoding/json"
	"fmt"
	"muhq.space/go/wrms/llog"
	"os/exec"
	"strings"
)

type YoutubeBackend struct {
	searchResults int
}

func NewYoutubeBackend() *YoutubeBackend {
	return &YoutubeBackend{10}
}

func (_ *YoutubeBackend) Play(song *Song, player *Player) {
	player.PlayUri("https://youtube.com/watch?v=" + song.Uri)
}

type YoutubeDlSearchResult struct {
	Id    string
	Title string
}

func (b *YoutubeBackend) Search(pattern string) []Song {
	searchOption := fmt.Sprintf("ytsearch%d:%s", b.searchResults, pattern)
	llog.Debug("Search youtube using: youtube-dl -j %s", searchOption)
	results, err := exec.Command("yt-dlp", "-j", searchOption).Output()

	if err != nil {
		llog.Error("youtube-dl failed with: %s", err)
	}

	songs := []Song{}
	for _, l := range strings.Split(string(results), "\n") {
		if l == "" {
			continue
		}

		result := YoutubeDlSearchResult{}
		err = json.Unmarshal([]byte(l), &result)
		if err != nil {
			llog.Debug("Parsing youtube-dl results '%s' failed", l)
			llog.Error("Parsing youtube-dl results failed with: %s", err)
		}
		songs = append(songs, NewSong(result.Title, "", "youtube", result.Id))
	}

	llog.Debug("youtube found %d matching videos", len(songs))
	return songs
}
