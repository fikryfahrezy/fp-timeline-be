package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/lesismal/nbio/nbhttp"
	"github.com/lesismal/nbio/nbhttp/websocket"
)

type Timeline struct {
	Id          int    `json:"id"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type TimelineMessage struct {
	Timeline
	Type string `json:"type"`
}

type wsHandler struct {
	mu        sync.Mutex
	timelines []Timeline
}

func (h *wsHandler) saveMewssage(data []byte) {
	var timelineMessage TimelineMessage
	err := json.Unmarshal(data, &timelineMessage)
	if err == nil {
		h.mu.Lock()
		defer h.mu.Unlock()

		timelineIndex := -1
		for i, timeline := range h.timelines {
			if timelineMessage.Id == timeline.Id {
				timelineIndex = i
			}
		}

		if timelineMessage.Type == "DELETE" {
			h.timelines = append(
				h.timelines[:timelineIndex],
				h.timelines[timelineIndex+1:]...,
			)
			return
		}

		newTimeline := Timeline{
			Id:          timelineMessage.Id,
			StartDate:   timelineMessage.StartDate,
			EndDate:     timelineMessage.EndDate,
			Title:       timelineMessage.Title,
			Description: timelineMessage.Description,
		}

		if timelineIndex != -1 {
			h.timelines[timelineIndex] = newTimeline
			return
		}

		h.timelines = append(h.timelines, newTimeline)
	}
}

func (h *wsHandler) newUpgrader() *websocket.Upgrader {
	u := websocket.NewUpgrader()

	u.OnMessage(func(c *websocket.Conn, messageType websocket.MessageType, data []byte) {
		h.saveMewssage(data)
		c.WriteMessage(messageType, data)
	})

	u.OnClose(func(c *websocket.Conn, err error) {
		fmt.Println("OnClose:", c.RemoteAddr().String(), err)
	})

	// allow all host
	u.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	return u
}

func (h *wsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upgrader := h.newUpgrader()
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("returning an error"))
		return
	}

	wsConn := conn.(*websocket.Conn)
	wsConn.SetReadDeadline(time.Time{})
	fmt.Println("OnOpen:", wsConn.RemoteAddr().String())
}

func main() {
	flag.Parse()
	mux := &http.ServeMux{}
	mux.Handle("/ws", new(wsHandler))

	svr := nbhttp.NewServer(nbhttp.Config{
		Network: "tcp",
		Addrs:   []string{"localhost:8888"},
		Handler: mux,
	})

	err := svr.Start()
	if err != nil {
		fmt.Printf("nbio.Start failed: %v\n", err)
		return
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	<-interrupt
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	svr.Shutdown(ctx)
}
