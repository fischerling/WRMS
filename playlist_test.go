package main

import "testing"

func TestPlAdd(t *testing.T) {
	var pl Playlist
	s := NewDummySong("Foo", "Bar")
	pl.Add(s)
	if len(pl) != 1 {
		t.Log("len should be 1")
		t.Fail()
	}
}

func TestPlAddPop(t *testing.T) {
	var pl Playlist
	s := NewDummySong("Foo", "Bar")
	pl.Add(s)
	if len(pl) != 1 {
		t.Log("len should be 1")
		t.Fail()
	}

	ps := pl.PopSong()
	if len(pl) != 0 {
		t.Log("len should be 0")
		t.Fail()
	}

	if ps != s {
		t.Log("the popped ptr should match the one added")
		t.Fail()
	}
}

func TestPlAddPopTwice(t *testing.T) {
	var pl Playlist
	s1 := NewDummySong("Foo", "Bar")
	s2 := NewDummySong("Nasen", "Baer")
	pl.Add(s1)
	pl.Add(s2)
	if len(pl) != 2 {
		t.Log("len should be 2")
		t.Fail()
	}

	ps1 := pl.PopSong()
	ps2 := pl.PopSong()
	if len(pl) != 0 {
		t.Log("len should be 0")
		t.Fail()
	}

	if ps1 != s1 {
		t.Log("the popped ptr should match the one added")
		t.Fail()
	}

	if ps2 != s2 {
		t.Log("the popped ptr should match the one added")
		t.Fail()
	}
}

func TestPlAdd2AdjustPop(t *testing.T) {
	var pl Playlist
	s1 := NewDummySong("Foo", "Bar")
	s2 := NewDummySong("Nasen", "Baer")
	pl.Add(s1)
	pl.Add(s2)
	if len(pl) != 2 {
		t.Log("len should be 2")
		t.Fail()
	}

	s2.Weight += 1
	pl.Adjust(s2)

	ps := pl.PopSong()
	if len(pl) != 1 {
		t.Log("len should be 1")
		t.Fail()
	}

	if ps != s2 {
		t.Log("the popped ptr should match the one adjusted")
		t.Fail()
	}
}

func TestPlAdd3AdjustAdjustPop(t *testing.T) {
	var pl Playlist
	s1 := NewDummySong("Foo", "Bar")
	s2 := NewDummySong("Nasen", "Baer")
	s3 := NewDummySong("Hut", "Traeger")
	pl.Add(s1)
	pl.Add(s2)
	pl.Add(s3)

	if len(pl) != 3 {
		t.Log("len should be 3")
		t.Fail()
	}

	s1.Weight += 1
	pl.Adjust(s1)

	s2.Weight -= 1
	pl.Adjust(s2)

	s3.Weight += 1
	pl.Adjust(s3)

	ps := pl.PopSong()
	if len(pl) != 2 {
		t.Log("len should be 2")
		t.Fail()
	}

	if ps == s2 {
		t.Log("the popped ptr is the downvoted one")
		t.Fail()
	}

	ps = pl.PopSong()
	if len(pl) != 1 {
		t.Log("len should be 1")
		t.Fail()
	}

	if ps == s2 {
		t.Log("the popped ptr is the downvoted one")
		t.Fail()
	}
}
