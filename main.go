package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"muhq.space/go/wrms/llog"

	"github.com/google/uuid"

	"nhooyr.io/websocket"
)

var wrms *Wrms
var pageTemplate *template.Template

func landingPage(w http.ResponseWriter, r *http.Request) {
	if _, err := r.Cookie("UUID"); err != nil {
		http.SetCookie(w, &http.Cookie{Name: "UUID", Value: uuid.NewString()})
	}

	if err := pageTemplate.Execute(w, wrms.Config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func getConnId(w http.ResponseWriter, r *http.Request) string {
	uuidCookie, err := r.Cookie("UUID")
	if err != nil {
		http.Error(w, "No connection ID cookie set", http.StatusUnauthorized)
		return ""
	}
	return uuidCookie.Value
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	connId := getConnId(w, r)
	if connId == "" {
		return
	}

	conn := wrms.GetConn(connId)

	pattern := r.URL.Query().Get("pattern")

	llog.Debug("Searching for %s", pattern)

	start := time.Now()
	resultsChan := wrms.Player.Search(pattern)

	go func() {
		for result := range resultsChan {
			if len(result) > 0 {
				conn.Send(Event{"search", result})
			}
		}

		conn.Send(newNotification("finish-search"))
		llog.Debug("searching for %s took %v", pattern, time.Since(start))
	}()

	fmt.Fprintf(w, "Starting search for %s", pattern)
}

func genericVoteHandler(w http.ResponseWriter, r *http.Request, vote string) {
	connId := getConnId(w, r)
	if connId == "" {
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
	data, err := ioutil.ReadAll(r.Body)
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
	wrms.Broadcast(Event{"add", []Song{song}})
	fmt.Fprintf(w, "Added song %s", string(data))
}

func playPauseHandler(w http.ResponseWriter, r *http.Request) {
	wrms.PlayPause()
}

func wsEndpoint(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		llog.Warning("Accepting the websocket failed with %s", err)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())

	ctx = c.CloseRead(ctx)

	uuidCookie, err := r.Cookie("UUID")
	if err != nil {
		llog.Error("websocket connection has no set UUID cookie")
	}

	id, err := uuid.Parse(uuidCookie.Value)
	if err != nil {
		llog.Error("Invalid UUID set in cookie")
	}

	defer func() {
		llog.Info("cancel context of connection %d", id)
		cancel()
	}()

	conn := &Connection{id, make(chan Event), c}
	wrms.Connections[id] = conn

	llog.Info("New websocket connection with id %d", id)

	initialCmds := [][]byte{}
	if wrms.Playing {
		data, err := json.Marshal(Event{"play", []Song{wrms.CurrentSong}})
		if err != nil {
			llog.Error("Encoding the play event failed with %s", err)
			return
		}
		initialCmds = append(initialCmds, data)
	}

	upvoted := []Song{}
	downvoted := []Song{}
	if len(wrms.Songs) > 0 {
		data, err := json.Marshal(Event{"add", wrms.Songs})
		if err != nil {
			llog.Error("Encoding the add event failed with %s", err)
			return
		}
		initialCmds = append(initialCmds, data)

		for _, song := range wrms.Songs {
			if _, ok := song.Upvotes[id.String()]; ok {
				upvoted = append(upvoted, song)
			} else if _, ok := song.Downvotes[id.String()]; ok {
				downvoted = append(downvoted, song)
			}
		}
	}

	if len(upvoted) > 0 {
		data, err := json.Marshal(Event{"upvoted", upvoted})
		if err != nil {
			llog.Error("Encoding the upvoted event failed with %s", err)
			return
		}
		initialCmds = append(initialCmds, data)
	}

	if len(downvoted) > 0 {
		data, err := json.Marshal(Event{"downvoted", downvoted})
		if err != nil {
			llog.Error("Encoding the upvoted event failed with %s", err)
			return
		}
		initialCmds = append(initialCmds, data)
	}

	for _, data := range initialCmds {
		llog.Info("Sending initial cmd %s", string(data))
		err = c.Write(ctx, websocket.MessageText, data)
		if err != nil {
			llog.Warning("Sending the initial commands failed with %s", err)
			return
		}
	}

	for ev := range conn.Events {
		data, err := json.Marshal(ev)
		if err != nil {
			llog.Error("Encoding the %s event failed with %s", ev.Event, err)
			return
		}

		llog.Debug("Sending ev %s to %s", string(data), id)
		err = c.Write(ctx, websocket.MessageText, data)
		if err != nil {
			llog.Warning("Sending the ev %s to %d failed with %s", string(data), id, err)
			return
		}
	}
}

func setupRoutes() {
	http.HandleFunc("/", landingPage)
	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/up", upHandler)
	http.HandleFunc("/down", downHandler)
	http.HandleFunc("/unvote", unvoteHandler)
	http.HandleFunc("/add", addHandler)
	http.HandleFunc("/playpause", playPauseHandler)
	http.HandleFunc("/ws", wsEndpoint)
}

func main() {
	config := Config{}
	logLevel := flag.String("loglevel", "Warning", "log level")
	flag.IntVar(&config.Port, "port", 8080, "port to listen to")
	flag.StringVar(&config.Backends, "backends", "dummy youtube spotify", "music backend to use")
	flag.StringVar(&config.LocalMusicDir, "serve-music-dir", "", "local music directory to serve")
	flag.StringVar(&config.UploadDir, "upload-dir", "uploads/", "directory to upload songs to")
	flag.Parse()

	llog.SetLogLevelFromString(*logLevel)
	config.HasUpload = strings.Contains(config.Backends, "upload")

	wrms = NewWrms(config)
	defer wrms.Close()

	setupRoutes()

	pageTemplate = template.Must(template.ParseFiles("web/client.html"))

	// wrms.AddSong(NewDummySong("Lala", "SNFMT"))
	// wrms.AddSong(NewDummySong("Hobelbank", "MC Wankwichtel"))

	err := http.ListenAndServe(fmt.Sprintf(":%d", config.Port), nil)
	llog.Error("Serving http failed with %s", err)
}
