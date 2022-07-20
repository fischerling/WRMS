package main

import (
	"container/heap"
)

type Playlist []*Song

func (pl Playlist) Len() int { return len(pl) }

func (pl Playlist) Less(i, j int) bool {
	return pl[i].Weight > pl[j].Weight
}

func (pl Playlist) Swap(i, j int) {
	pl[i], pl[j] = pl[j], pl[i]
	pl[i].index = i
	pl[j].index = j
}

func (pl *Playlist) Push(x any) {
	n := len(*pl)
	song := x.(*Song)
	song.index = n
	*pl = append(*pl, song)
}

func (pl *Playlist) Pop() any {
	old := *pl
	n := len(old)
	s := old[n-1]
	old[n-1] = nil // avoid memory leak
	s.index = -1   // for safety
	*pl = old[0 : n-1]
	return s
}

func (pl *Playlist) PopSong() *Song {
	return heap.Pop(pl).(*Song)
}

func (pl *Playlist) Add(s *Song) {
	heap.Push(pl, s)
}

func (pl *Playlist) Adjust(s *Song) {
	heap.Fix(pl, s.index)
}
