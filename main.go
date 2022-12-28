package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"

	"muhq.space/go/wrms/llog"

	"github.com/google/uuid"

	"nhooyr.io/websocket"
)

var wrms *Wrms

func landingPage(w http.ResponseWriter, r *http.Request) {
	if _, err := r.Cookie("UUID"); err != nil {
		http.SetCookie(w, &http.Cookie{Name: "UUID", Value: uuid.NewString()})
	}

	t, err := template.ParseFiles("web/client.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t.Execute(w, "")
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	pattern := r.URL.Query().Get("pattern")

	llog.Debug(fmt.Sprintf("Searching for %s", pattern))

	results := wrms.Player.Search(pattern)
	data, err := json.Marshal(Event{"search", results})
	if err != nil {
		llog.Error(fmt.Sprintf("Encoding the search result failed with %s", err))
		return
	}

	llog.Info(fmt.Sprintf("Search returned: %s", string(data)))
	w.Write(data)
}

func genericVoteHandler(w http.ResponseWriter, r *http.Request, vote string) {
	uuidCookie, err := r.Cookie("UUID")
	if err != nil {
		http.Error(w, "No connection ID cookie set", http.StatusUnauthorized)
		return
	}
	connId := uuidCookie.Value

	songUri := r.URL.Query().Get("song")
	llog.Info(fmt.Sprintf("%s song %s via url %s", vote, songUri, r.URL))
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
		llog.Warning(fmt.Sprintf("Failed to read request body: %s", string(data)))
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	song, err := NewSongFromJson(data)
	if err != nil {
		http.Error(w, "Could not parse song", http.StatusBadRequest)
		return
	}

	llog.Info(fmt.Sprintf("Added song %s", string(data)))
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
		llog.Warning(fmt.Sprintf("Accepting the websocket failed with %s", err))
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
		llog.Info(fmt.Sprintf("cancel context of connection %d", id))
		cancel()
	}()

	conn := Connection{id, make(chan Event), c}
	wrms.Connections = append(wrms.Connections, conn)

	llog.Info(fmt.Sprintf("New websocket connection with id %d", id))

	initialCmds := [][]byte{}
	if wrms.Playing {
		data, err := json.Marshal(Event{"play", []Song{wrms.CurrentSong}})
		if err != nil {
			llog.Error(fmt.Sprintf("Encoding the play event failed with %s", err))
			return
		}
		initialCmds = append(initialCmds, data)
	}

	upvoted := []Song{}
	downvoted := []Song{}
	if len(wrms.Songs) > 0 {
		data, err := json.Marshal(Event{"add", wrms.Songs})
		if err != nil {
			llog.Error(fmt.Sprintf("Encoding the add event failed with %s", err))
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
			llog.Error(fmt.Sprintf("Encoding the upvoted event failed with %s", err))
			return
		}
		initialCmds = append(initialCmds, data)
	}

	if len(downvoted) > 0 {
		data, err := json.Marshal(Event{"downvoted", downvoted})
		if err != nil {
			llog.Error(fmt.Sprintf("Encoding the upvoted event failed with %s", err))
			return
		}
		initialCmds = append(initialCmds, data)
	}

	for _, data := range initialCmds {
		llog.Info(fmt.Sprintf("Sending initial cmd %s", string(data)))
		err = c.Write(ctx, websocket.MessageText, data)
		if err != nil {
			llog.Warning(fmt.Sprintf("Sending the initial commands failed with %s", err))
			return
		}
	}

	for ev := range conn.Events {
		data, err := json.Marshal(ev)
		if err != nil {
			llog.Error(fmt.Sprintf("Encoding the %s event failed with %s", ev.Event, err))
			return
		}

		llog.Debug(fmt.Sprintf("Sending ev %s to %s", string(data), id))
		err = c.Write(ctx, websocket.MessageText, data)
		if err != nil {
			llog.Warning(fmt.Sprintf("Sending the ev %s to %d failed with %s", string(data), id, err))
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
	http.Handle("/static/", http.FileServer(http.Dir("./web")))
}

func main() {
	config := Config{}
	logLevel := flag.String("loglevel", "Warning", "log level")
	flag.IntVar(&config.port, "port", 8080, "port to listen to")
	flag.StringVar(&config.backends, "backends", "dummy youtube spotify", "music backend to use")
	flag.StringVar(&config.localMusicDir, "serve-music-dir", "", "local music directory to serve")
	flag.Parse()

	llog.SetLogLevelFromString(*logLevel)

	wrms = NewWrms(config)
	defer wrms.Close()

	setupRoutes()

	// wrms.AddSong(NewDummySong("Lala", "SNFMT"))
	// wrms.AddSong(NewDummySong("Hobelbank", "MC Wankwichtel"))

	err := http.ListenAndServe(fmt.Sprintf(":%d", config.port), nil)
	llog.Error(fmt.Sprintf("Serving http failed with %s", err))
}
