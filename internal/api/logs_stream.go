package api

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sahithvibudhi/flightdeck/internal/db"
	"github.com/sahithvibudhi/flightdeck/internal/process"
)

const (
	streamPollInterval = 500 * time.Millisecond
	streamHeartbeat    = 15 * time.Second
)

/*
LogsStream tails an app's log over Server-Sent Events. It sends the
last 100 lines immediately, then follows the file, so the dashboard
shows output in near-real-time instead of polling snapshots.
*/
func (h *AppsHandler) LogsStream(w http.ResponseWriter, r *http.Request) {
	app, err := db.GetApp(h.database, chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming not supported"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	var offset int64
	if lines, err := process.TailLog(app.LogPath, 100); err == nil {
		for _, line := range lines {
			sendEvent(w, line)
		}
		if info, err := os.Stat(app.LogPath); err == nil {
			offset = info.Size()
		}
		flusher.Flush()
	}

	poll := time.NewTicker(streamPollInterval)
	defer poll.Stop()
	heartbeat := time.NewTicker(streamHeartbeat)
	defer heartbeat.Stop()

	var pending string
	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case <-poll.C:
			info, err := os.Stat(app.LogPath)
			if err != nil {
				continue
			}
			if info.Size() < offset {
				// The log was truncated (capLog); start over from the top.
				offset = 0
				pending = ""
			}
			if info.Size() == offset {
				continue
			}

			f, err := os.Open(app.LogPath)
			if err != nil {
				continue
			}
			buf := make([]byte, info.Size()-offset)
			n, _ := f.ReadAt(buf, offset)
			f.Close()
			if n <= 0 {
				continue
			}
			offset += int64(n)

			chunk := pending + string(buf[:n])
			lines := strings.Split(chunk, "\n")
			// The last element is either "" (chunk ended in newline) or a
			// partial line to carry into the next read.
			pending = lines[len(lines)-1]
			for _, line := range lines[:len(lines)-1] {
				sendEvent(w, line)
			}
			flusher.Flush()
		}
	}
}

func sendEvent(w http.ResponseWriter, line string) {
	fmt.Fprintf(w, "data: %s\n\n", line)
}
