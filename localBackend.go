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
		Artist text,
		Album text,
		Year int
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

	stmt, err := tx.Prepare("INSERT INTO songs(Uri, Title, Artist, Album, Year) VALUES(?, ?, ?, ?, ?)")
	if err != nil {
		llog.Fatal("Preparing inser statement failed: %q", err)
	}
	defer stmt.Close()

	for _, song := range songs {
		_, err = stmt.Exec(song.Uri, song.Title, song.Artist, song.Album, song.Year)
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
		s.Album = m.Album()
		s.Year = m.Year()
		songs = append(songs, s)
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

func genericQuery(pattern string) string {
	return fmt.Sprintf("SELECT * FROM songs WHERE Title LIKE '%%%s%%' OR Artist LIKE '%%%s%%' OR Album LIKE '%%%s%%'", pattern, pattern, pattern)
}

func advancedQuery(patterns map[string]string) string {
	query := "Select * FROM songs WHERE"
	query_parts := []string{}
	for _, comp := range []string{"title", "album", "artist"} {
		if pattern, ok := patterns[comp]; ok {
			titleComp := strings.Title(comp) //nolint:staticcheck
			query_parts = append(query_parts, fmt.Sprintf("%s LIKE '%%%s%%'", titleComp, pattern))
		}
	}

	return fmt.Sprintf("%s %s", query, strings.Join(query_parts, " OR "))
}

func (b *LocalBackend) Search(patterns map[string]string) (results []*Song) {
	advanced := false
	for _, comp := range []string{"title", "album", "artist"} {
		if _, ok := patterns[comp]; ok {
			advanced = true
			break
		}
	}

	var query string
	if advanced {
		query = advancedQuery(patterns)
	} else {
		query = genericQuery(strings.ToLower(patterns["pattern"]))
	}

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
		var album string
		var year int

		err = rows.Scan(&uri, &title, &artist, &album, &year)
		if err != nil {
			llog.Warning("Scanning query result failed: %q", err)
		}

		s := NewDetailedSong(title, artist, "local", uri, album, year)
		results = append(results, s)
	}

	err = rows.Err()
	if err != nil {
		llog.Error("Iterator returned error: %q", err)
	}

	llog.Debug("Local search returned %d results", len(results))

	return results
}
