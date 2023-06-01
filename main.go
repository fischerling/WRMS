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

	"nhooyr.io/websocket"
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

	pattern := r.URL.Query().Get("pattern")
	if pattern == "" {
		http.Error(w, "No search pattern provided", http.StatusBadRequest)
		return
	}

	id := wrms.eventId.Add(1)

	llog.Debug("Searching for %s", pattern)

	start := time.Now()
	resultsChan := wrms.Player.Search(pattern)

	go func() {
		for result := range resultsChan {
			if len(result) > 0 {
				conn.Send(Event{Event: "search", Id: id, Songs: result})
			}
		}

		conn.Send(Event{Event: "finish-search", Id: id})
		llog.Debug("searching for %s took %v", pattern, time.Since(start))
	}()

	fmt.Fprintf(w, "Starting search for %s", pattern)
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

func wsEndpoint(w http.ResponseWriter, r *http.Request) {
	connId, err := getConnId(w, r)
	if err != nil {
		return
	}

	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		llog.Warning("Accepting the websocket failed with %s", err)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	ctx = ws.CloseRead(ctx)

	defer func() {
		llog.Info("cancel context of connection %v", connId)
		cancel()
	}()

	conn := newConnection(connId, ws, ctx, cancel)
	defer conn.Close()

	llog.Info("New websocket connection with id %v", connId)
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
	http.HandleFunc("/ws", wsEndpoint)
}

func main() {
	config := newConfig()

	flag.StringVar(&config.LogLevel, "loglevel", config.LogLevel, "log level")
	flag.IntVar(&config.Port, "port", config.Port, "port to listen to")
	backends := flag.String("backends", "", "music backend to use")
	flag.StringVar(&config.LocalMusicDir,
		"serve-music-dir", config.LocalMusicDir, "local music directory to serve")
	flag.StringVar(&config.UploadDir, "upload-dir", config.UploadDir, "directory to upload songs to")
	genAdmin := flag.Bool("generate-admin", false, "generate an admin uuid")
	flag.Parse()

	if *backends != "" {
		config.Backends = strings.Split(*backends, " ")
	}

	if *genAdmin {
		var err error
		config.Admin, err = uuid.NewRandom()
		if err != nil {
			llog.Fatal("Failed to create random UUID: %v", err)
		}
		llog.Info("Admin uuid: %v", config.Admin)
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
