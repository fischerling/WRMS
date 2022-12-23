package main

import (
	"fmt"
	"github.com/AnjanaMadu/YTSearch"
	"muhq.space/go/wrms/llog"
)

type YoutubeBackend struct{}

func (youtube *YoutubeBackend) Play(song *Song, player *Player) {
	player.PlayUri("https://youtube.com/watch?v=" + song.Uri)
}

func (youtube *YoutubeBackend) Search(pattern string) []Song {
	results, err := ytsearch.Search(pattern)
	if err != nil {
		llog.Error(fmt.Sprintf("Youtube search failed with", err))
	}

	songs := []Song{}
	for i, result := range results {
		if i == 10 {
			break
		}

		if result.VideoId == "" {
			continue
		}

		songs = append(songs, NewSong(result.Title, "", "youtube", result.VideoId))
	}

	llog.Debug(fmt.Sprintf("youtube found %d matching videos", len(songs)))
	return songs
}
