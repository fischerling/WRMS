package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"

	"muhq.space/go/wrms/llog"

	"github.com/google/uuid"
)

type Connection struct {
	wrms      *Wrms
	Id        uuid.UUID
	nr        uint64
	closing   atomic.Bool
	refs      atomic.Int64
	Events    chan Event
	w         http.ResponseWriter
	flusher   http.Flusher
	ctx       context.Context
	cancel    func()
	nextEvent uint64
}

const EVENT_BUFFER_SIZE = 3

func (wrms *Wrms) newConnection(id uuid.UUID,
	w http.ResponseWriter,
	ctx context.Context,
	cncl func()) *Connection {
	// prepare the flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		llog.Fatal("Flusher type assertion failed")
	}

	conn := &Connection{
		wrms:    wrms,
		Id:      id,
		nr:      wrms.nextConnNr.Add(1),
		Events:  make(chan Event, EVENT_BUFFER_SIZE),
		w:       w,
		flusher: flusher,
		ctx:     ctx,
		cancel:  cncl,
	}

	wrms.addConn(conn)
	return conn
}

func (c *Connection) Send(ev Event) {
	llog.DDebug("Sending %v to %v", ev, c.Id)
	// Connection is closing -> not write to it
	if c.closing.Load() {
		return
	}
	// Register us as sender
	c.refs.Add(1)
	// Send the event
	c.Events <- ev
	// Deregister us as sender
	c.refs.Add(-1)
}

func (conn *Connection) _send(evs []interface{}) error {
	for _, ev := range evs {
		data, err := json.Marshal(ev)
		if err != nil {
			llog.Warning("Encoding the initial command %v failed with %s", ev, err)
			return err
		}

		sdata := string(data)
		llog.Debug("Sending ev %s to %s", sdata, conn.Id)
		fmt.Fprintf(conn.w, "data: %s\n\n", sdata)
		conn.flusher.Flush()
	}

	return nil
}

func (conn *Connection) _sendEv(ev Event) error {
	return conn._send([]interface{}{ev})
}

func (conn *Connection) serve() {
	// Map to store future events to send them in order
	toSend := make(map[uint64]*Event)

	for {
		var ev Event
		// We have the next event -> send it
		if toSend[conn.nextEvent] != nil {
			ev = *toSend[conn.nextEvent]
			delete(toSend, conn.nextEvent)
			// We have not received the next event yet -> receive a new event
		} else {
			llog.DDebug("%v: Awaiting event %d", conn.Id, conn.nextEvent)
			select {
			case ev = <-conn.Events:
			case <-conn.ctx.Done():
				return
			}
			// We received a future object -> store it and continue
			if ev.Id > conn.nextEvent {
				llog.DDebug("%v: Received future event %d", conn.Id, ev.Id)
				toSend[ev.Id] = &ev
				continue
			}
		}

		// increment the expected event id
		if ev.Id == conn.nextEvent {
			conn.nextEvent++
		}

		conn._sendEv(ev)
	}
}

func (c *Connection) Close() {
	llog.Info("Closing connection %s", c.Id)
	// Remove the closing connection from the map
	wrms.delConn(c)
	// Announce that the connection is going to be closed
	c.closing.Store(true)

	// Consume all events from the registered senders
	for c.refs.Load() > 0 {
		select {
		case <-c.Events:
		default:
		}
	}

	close(c.Events)
}
