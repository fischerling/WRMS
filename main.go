package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/google/uuid"

	"nhooyr.io/websocket"
)

var wrms = NewWrms()

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

	log.Println(fmt.Sprintf("Searching for %s", pattern))

	results := wrms.Player.Search(pattern)
	data, err := json.Marshal(Event{"search", results})
	if err != nil {
		log.Fatal(err)
		return
	}
	w.Write(data)
}

func genericVoteHandler(w http.ResponseWriter, r *http.Request, vote string) {
	uuidCookie, err := r.Cookie("UUID")
	if err != nil {
		http.Error(w, "No connection ID cookie set", http.StatusUnauthorized)
		return
	}
	connId := uuidCookie.Value

	songId := r.URL.Query().Get("song")
	log.Println("%s song %s via url %s", vote, songId, r.URL)
	wrms.AdjustSongWeight(connId, songId, vote)
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
	fmt.Fprintf(w, "Added")
}

func playPauseHandler(w http.ResponseWriter, r *http.Request) {
	wrms.PlayPause()
}

func wsEndpoint(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Println("%v", err)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())

	ctx = c.CloseRead(ctx)

	uuidCookie, err := r.Cookie("UUID")
	if err != nil {
		log.Fatal("websocket connection has no set UUID cookie")
	}

	id, err := uuid.Parse(uuidCookie.Value)
	if err != nil {
		log.Fatal("Invalid UUID set in cookie")
	}

	defer func() {
		log.Println("cancel context of connection", id)
		cancel()
	}()

	conn := Connection{id, make(chan Event), c}
	wrms.Connections = append(wrms.Connections, conn)

	log.Println("New websocket connection with id", id)

	initialCmds := [][]byte{}
	if wrms.Playing {
		data, err := json.Marshal(Event{"play", []Song{*wrms.CurrentSong}})
		if err != nil {
			log.Fatal(err)
			return
		}
		initialCmds = append(initialCmds, data)
	}

	data, err := json.Marshal(Event{"add", wrms.Songs})
	if err != nil {
		log.Fatal(err)
		return
	}
	initialCmds = append(initialCmds, data)

	for _, data := range initialCmds {
		log.Println("Sending initial cmd", string(data))
		err = c.Write(ctx, websocket.MessageText, data)
		if err != nil {
			log.Println(err)
			return
		}
	}

	for ev := range conn.Events {
		data, err := json.Marshal(ev)
		if err != nil {
			log.Fatal(err)
			return
		}

		log.Println(fmt.Sprintf("Sending ev %s to %s", string(data), id))
		err = c.Write(ctx, websocket.MessageText, data)
		if err != nil {
			log.Println(err)
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
	setupRoutes()

	wrms.AddSong(NewSong("Lala", "SNFMT", "local"))
	wrms.AddSong(NewSong("Hobelbank", "MC Wankwichtel", "local"))

	log.Println(wrms)
	defer wrms.Close()
	log.Fatal(http.ListenAndServe(":8080", nil))
}
