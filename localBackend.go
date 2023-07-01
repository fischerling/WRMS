package main

import (
	"database/sql"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/dhowden/tag"
	_ "github.com/mattn/go-sqlite3"
	"muhq.space/go/wrms/llog"
)

const DB_URL = "file:songs?mode=memory&cache=shared"

type LocalBackend struct {
	musicDir string
	db       *sql.DB
}

func NewLocalBackend(musicDir string) *LocalBackend {
	b := LocalBackend{musicDir: musicDir}

	var err error
	b.db, err = sql.Open("sqlite3", DB_URL)
	if err != nil {
		llog.Fatal("Opening in-memory db failed: %q", err)
	}

	sqlStmt := `
	CREATE TABLE songs (
		Uri text NOT NULL PRIMARY KEY,
		Title text,
		Artist text
	);
	`
	_, err = b.db.Exec(sqlStmt)
	if err != nil {
		llog.Fatal("%q: %s", err, sqlStmt)
	}

	go b.findSongs()
	return &b
}

func (_ *LocalBackend) OnSongFinished(*Song) {}

func (b *LocalBackend) insert(songs []*Song) {
	llog.Debug("Inserting %d songs into local DB", len(songs))
	tx, err := b.db.Begin()
	if err != nil {
		llog.Fatal("Starting insert transaction failed: %q", err)
	}

	stmt, err := tx.Prepare("INSERT INTO songs(Uri, Title, Artist) VALUES(?, ?, ?)")
	if err != nil {
		llog.Fatal("Preparing inser statement failed: %q", err)
	}
	defer stmt.Close()

	for _, song := range songs {
		_, err = stmt.Exec(song.Uri, song.Title, song.Artist)
		if err != nil {
			llog.Fatal("Executing insert statement failed: %q", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		llog.Fatal("Commiting insert transaction failed: %q", err)
	}
}

func (b *LocalBackend) findSongs() {
	llog.Debug("Starting song search under: %s", b.musicDir)

	var songs []*Song

	err := filepath.Walk(b.musicDir, func(p string, finfo os.FileInfo, err error) error {
		if err != nil {
			llog.Warning("error %v at path %s\n", err, p)
			return err
		}

		if !finfo.Mode().IsRegular() {
			return nil
		}

		ext := strings.ToLower(path.Ext(p))
		for _, excluded := range []string{".png", ".jpg", ".txt", ".pdf", ".m3u"} {
			if ext == excluded {
				return nil
			}
		}

		f, err := os.Open(p)
		if err != nil {
			llog.Warning("error %v opening file %s", err, p)
			return err
		}
		defer f.Close()

		m, err := tag.ReadFrom(f)
		if err != nil {
			llog.Warning("error reading tags from %s: %v", p, err)
			// Contiune walking
			return nil
		}

		s := NewSong(m.Title(), m.Artist(), "local", p)
		songs = append(songs, &s)
		return nil
	})

	if err != nil {
		llog.Error("error walking the path %q: %v", b.musicDir, err)
	}

	b.insert(songs)
}

func (b *LocalBackend) Play(song *Song, player Player) {
	player.PlayUri("file://" + song.Uri)
}

func (b *LocalBackend) Search(keyword string) (results []Song) {
	pattern := strings.ToLower(keyword)

	query := fmt.Sprintf("SELECT * FROM songs WHERE Title LIKE '%%%s%%' OR Artist LIKE '%%%s%%'",
		pattern, pattern)
	llog.Debug("Searching in local DB using: %q", query)
	rows, err := b.db.Query(query)
	if err != nil {
		llog.Error("Building search query %q failed: %q", query, err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var uri string
		var title string
		var artist string

		err = rows.Scan(&uri, &title, &artist)
		if err != nil {
			llog.Warning("Scanning query result failed: %q", err)
		}

		results = append(results, NewSong(title, artist, "local", uri))
	}

	err = rows.Err()
	if err != nil {
		llog.Error("Iterator returned error: %q", err)
	}

	llog.Debug("Local search returned %d results", len(results))

	return results
}
