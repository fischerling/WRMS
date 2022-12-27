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
	llog.Debug(fmt.Sprintf("Search youtube using: youtube-dl -j %s", searchOption))
	results, err := exec.Command("youtube-dl", "-j", searchOption).Output()

	if err != nil {
		llog.Error(fmt.Sprintf("youtube-dl failed with: %s", err))
	}

	songs := []Song{}
	for _, l := range strings.Split(string(results), "\n") {
		if l == "" {
			continue
		}

		result := YoutubeDlSearchResult{}
		err = json.Unmarshal([]byte(l), &result)
		if err != nil {
			llog.Debug(fmt.Sprintf("Parsing youtube-dl results '%s' failed", l))
			llog.Error(fmt.Sprintf("Parsing youtube-dl results failed with: %s", err))
		}
		songs = append(songs, NewSong(result.Title, "", "youtube", result.Id))
	}

	llog.Debug(fmt.Sprintf("youtube found %d matching videos", len(songs)))
	return songs
}
