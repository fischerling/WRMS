package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/exp/slices"
	"muhq.space/go/wrms/llog"

	"github.com/google/uuid"
)

var wrms *Wrms
var pageTemplate *template.Template

func landingPage(w http.ResponseWriter, r *http.Request) {
	var id uuid.UUID
	if cookie, err := r.Cookie("UUID"); err != nil {
		id, err = uuid.NewRandom()
		if err != nil {
			llog.Fatal("Failed to generate random uuid: %v", err)
		}

		http.SetCookie(w, &http.Cookie{Name: "UUID", Value: id.String()})
	} else {
		id, err = uuid.Parse(cookie.Value)
		if err != nil {
			http.Error(w, "Invalid UUID set in cookie", http.StatusBadRequest)
			return
		}
	}

	tempData := struct {
		Config  Config
		IsAdmin bool
	}{wrms.Config, wrms.Config.IsAdmin(id)}

	if err := pageTemplate.Execute(w, tempData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func getConnId(w http.ResponseWriter, r *http.Request) (uuid.UUID, error) {
	uuidCookie, err := r.Cookie("UUID")
	if err != nil {
		http.Error(w, "No connection ID cookie set", http.StatusUnauthorized)
		return uuid.Nil, err
	}

	id, err := uuid.Parse(uuidCookie.Value)
	if err != nil {
		http.Error(w, "Invalid UUID set in cookie", http.StatusBadRequest)
		return uuid.Nil, err
	}

	return id, nil
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	connId, err := getConnId(w, r)
	if err != nil {
		return
	}

	conn := wrms.GetConn(connId)
	if conn == nil {
		http.Error(w, "No websocket connection found", http.StatusInternalServerError)
		return
	}

	searchQuery := map[string]string{}
	patterns := []string{"pattern", "title", "album", "artist"}
	for _, pattern := range patterns {
		value := r.URL.Query().Get(pattern)
		if value != "" {
			searchQuery[pattern] = value
		}
	}

	if len(searchQuery) == 0 {
		http.Error(w, "No search pattern provided", http.StatusBadRequest)
		return
	}

	id := wrms.eventId.Add(1)

	llog.Debug("Searching for %v", searchQuery)

	start := time.Now()
	resultsChan := wrms.Search(searchQuery)

	go func() {
		for result := range resultsChan {
			if len(result) > 0 {
				conn.Send(Event{Event: "search", Id: id, Songs: result})
			}
		}

		conn.Send(Event{Event: "finish-search", Id: id})
		llog.Debug("searching for %v took %v", searchQuery, time.Since(start))
	}()

	fmt.Fprintf(w, "Starting search for %v", searchQuery)
}

func genericVoteHandler(w http.ResponseWriter, r *http.Request, vote string) {
	connId, err := getConnId(w, r)
	if err != nil {
		return
	}

	songUri := r.URL.Query().Get("song")
	llog.Info("%s song %s via url %s", vote, songUri, r.URL)
	wrms.AdjustSongWeight(connId, songUri, vote)
}

func upHandler(w http.ResponseWriter, r *http.Request) {
	genericVoteHandler(w, r, "up")
}

func downHandler(w http.ResponseWriter, r *http.Request) {
	genericVoteHandler(w, r, "down")
}

func unvoteHandler(w http.ResponseWriter, r *http.Request) {
	genericVoteHandler(w, r, "unvote")
}

func addHandler(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		llog.Warning("Failed to read request body: %s", string(data))
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	song, err := NewSongFromJson(data)
	if err != nil {
		http.Error(w, "Could not parse song", http.StatusBadRequest)
		return
	}

	wrms.AddSong(song)
	fmt.Fprintf(w, "Added song %s", string(data))
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	connId, err := getConnId(w, r)
	if err != nil {
		return
	}

	if !wrms.Config.IsAdmin(connId) {
		http.Error(w, "Only admins are allowed to delete songs", http.StatusUnauthorized)
		return
	}

	songUri := r.URL.Query().Get("song")
	llog.Info("Delete song %s via url %s", songUri, r.URL)
	wrms.DeleteSong(songUri)
}

func genericControlHandler(w http.ResponseWriter, r *http.Request, cmd string) {
	connId, err := getConnId(w, r)
	if err != nil {
		return
	}

	if !wrms.Config.IsAdmin(connId) {
		http.Error(w, "Only admins are allowed to control playback", http.StatusUnauthorized)
		return
	}

	switch cmd {
	case "playpause":
		wrms.PlayPause()
	case "next":
		wrms.Next()
	}
}

func playPauseHandler(w http.ResponseWriter, r *http.Request) {
	genericControlHandler(w, r, "playpause")
}

func nextHandler(w http.ResponseWriter, r *http.Request) {
	genericControlHandler(w, r, "next")
}

func adminHandler(w http.ResponseWriter, r *http.Request) {
	connId, err := getConnId(w, r)
	if err != nil {
		return
	}

	_pw, err := io.ReadAll(r.Body)
	if err != nil {
		llog.Warning("Failed to read admin password from request body")
		http.Error(w, "Failed to read asdmin password from request body", http.StatusInternalServerError)
		return
	}

	pw := string(_pw)

	if pw == wrms.Config.AdminPW {
		wrms.Config.Admins = append(wrms.Config.Admins, connId)
	} else {
		llog.Warning("Wrong admin password: %s", pw)
		http.Error(w, "Wrong admin password", http.StatusUnauthorized)
		return
	}
}

func eventsEndpoint(w http.ResponseWriter, r *http.Request) {
	connId, err := getConnId(w, r)
	if err != nil {
		return
	}

	// prepare the header
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ctx, cancel := context.WithCancel(r.Context())

	conn := newConnection(connId, w, ctx, cancel)

	defer func() {
		llog.Info("cancel context of connection %v", connId)
		cancel()
		conn.Close()
	}()

	llog.Info("New SSE connection with id %v", connId)
	err = wrms.initConn(conn)
	if err != nil {
		return
	}

	conn.serve()
}

func setupRoutes() {
	http.HandleFunc("/", landingPage)
	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/up", upHandler)
	http.HandleFunc("/down", downHandler)
	http.HandleFunc("/unvote", unvoteHandler)
	http.HandleFunc("/add", addHandler)
	http.HandleFunc("/delete", deleteHandler)
	http.HandleFunc("/next", nextHandler)
	http.HandleFunc("/playpause", playPauseHandler)
	http.HandleFunc("/admin", adminHandler)
	http.HandleFunc("/events", eventsEndpoint)
}

func main() {
	config := newConfig()

	flag.StringVar(&config.LogLevel, "loglevel", config.LogLevel, "log level")
	flag.IntVar(&config.Port, "port", config.Port, "port to listen to")
	backends := flag.String("backends", "", "music backend to use")
	flag.StringVar(&config.LocalMusicDir,
		"serve-music-dir", config.LocalMusicDir, "local music directory to serve")
	flag.StringVar(&config.UploadDir, "upload-dir", config.UploadDir, "directory to upload songs to")
	flag.Parse()

	if *backends != "" {
		config.Backends = strings.Split(*backends, " ")
	}

	llog.SetLogLevelFromString(config.LogLevel)
	config.HasUpload = slices.Contains(config.Backends, "upload")

	wrms = NewWrms(config)

	setupRoutes()

	pageTemplate = template.Must(template.ParseFiles("web/client.html"))

	// wrms.AddSong(NewDummySong("Lala", "SNFMT"))
	// wrms.AddSong(NewDummySong("Hobelbank", "MC Wankwichtel"))

	llog.Info("Serving http on %d", config.Port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", config.Port), nil)
	llog.Error("Serving http failed with %s", err)
}
