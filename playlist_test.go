package main

import "testing"

func TestAdd(t *testing.T) {
	var pl Playlist
	s := NewDummySong("Foo", "Bar")
	pl.Add(&s)
	if len(pl) != 1 {
		t.Log("len should be 1")
		t.Fail()
	}
}

func TestAddPop(t *testing.T) {
	var pl Playlist
	s := NewDummySong("Foo", "Bar")
	pl.Add(&s)
	if len(pl) != 1 {
		t.Log("len should be 1")
		t.Fail()
	}

	ps := pl.PopSong()
	if len(pl) != 0 {
		t.Log("len should be 0")
		t.Fail()
	}

	if ps != &s {
		t.Log("the popped ptr should match the one added")
		t.Fail()
	}
}

func TestAddPopTwice(t *testing.T) {
	var pl Playlist
	s1 := NewDummySong("Foo", "Bar")
	s2 := NewDummySong("Nasen", "Baer")
	pl.Add(&s1)
	pl.Add(&s2)
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

	if ps1 != &s1 {
		t.Log("the popped ptr should match the one added")
		t.Fail()
	}

	if ps2 != &s2 {
		t.Log("the popped ptr should match the one added")
		t.Fail()
	}
}

func TestAdd2AdjustPop(t *testing.T) {
	var pl Playlist
	s1 := NewDummySong("Foo", "Bar")
	s2 := NewDummySong("Nasen", "Baer")
	pl.Add(&s1)
	pl.Add(&s2)
	if len(pl) != 2 {
		t.Log("len should be 2")
		t.Fail()
	}

	s2.Weight += 1
	pl.Adjust(&s2)

	ps := pl.PopSong()
	if len(pl) != 1 {
		t.Log("len should be 1")
		t.Fail()
	}

	if ps != &s2 {
		t.Log("the popped ptr should match the one adjusted")
		t.Fail()
	}
}
