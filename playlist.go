package main

import (
	"container/heap"
	"muhq.space/go/wrms/llog"
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
	llog.DDebug("popping song from the playlist (%p) -> %v", pl, pl)
	if len(*pl) == 0 {
		return nil
	}

	s := heap.Pop(pl).(*Song)
	llog.DDebug("popped song %p from the playlist (%p) -> %v", s, pl, pl)
	return s
}

func (pl *Playlist) Add(s *Song) {
	heap.Push(pl, s)
	llog.DDebug("added song %p to the playlist (%p) -> %v", s, pl, pl)
}

func (pl *Playlist) Adjust(s *Song) {
	heap.Fix(pl, s.index)
	llog.DDebug("adjusting song %p in the playlist (%p) -> %v", s, pl, pl)
}

func (pl *Playlist) RemoveSong(s *Song) {
	heap.Remove(pl, s.index)
	llog.DDebug("removing song %p in the playlist (%p) -> %v", s, pl, pl)
}

func (pl *Playlist) OrderedList() []*Song {
	songs := make([]*Song, 0, pl.Len())

	cpy := make(Playlist, 0, pl.Len())
	copy(cpy, *pl)

	for cpy.Len() > 0 {
		songs = append(songs, heap.Pop(&cpy).(*Song))
	}

	return songs
}

func (pl *Playlist) applyTimeBonus(timeBonus float64) {
	for _, s := range *pl {
		s.Weight += timeBonus
	}
	heap.Init(pl)
	llog.DDebug("applying time bonus to the playlist")
}
