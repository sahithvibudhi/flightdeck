package api

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sahithvibudhi/flightdeck/internal/db"
)

/*
Deploy logs are buffered in memory while a deploy runs so clients can
watch it live, then persisted to the deployments table when it finishes.
One buffer per running deployment, owned by the AppsHandler's hub.
*/
type deployLogHub struct {
	mu   sync.Mutex
	bufs map[string]*deployLogBuf
}

func newDeployLogHub() *deployLogHub {
	return &deployLogHub{bufs: make(map[string]*deployLogBuf)}
}

type deployLogBuf struct {
	mu      sync.Mutex
	lines   []string
	partial string
	subs    map[chan string]struct{}
	done    chan struct{}
}

func (h *deployLogHub) start(depID string) *deployLogBuf {
	b := &deployLogBuf{
		subs: make(map[chan string]struct{}),
		done: make(chan struct{}),
	}
	h.mu.Lock()
	h.bufs[depID] = b
	h.mu.Unlock()
	return b
}

func (h *deployLogHub) get(depID string) *deployLogBuf {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.bufs[depID]
}

// finish closes the buffer to new subscribers and removes it from the
// hub. Callers persist the text to the database first, so late readers
// fall back to the stored log.
func (h *deployLogHub) finish(depID string) {
	h.mu.Lock()
	b := h.bufs[depID]
	delete(h.bufs, depID)
	h.mu.Unlock()
	if b == nil {
		return
	}
	b.mu.Lock()
	close(b.done)
	b.mu.Unlock()
}

// Write implements io.Writer: input is split into lines, buffered, and
// fanned out to live subscribers. A trailing fragment without a newline
// is held back until it completes.
func (b *deployLogBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	text := b.partial + string(p)
	lines := strings.Split(text, "\n")
	b.partial = lines[len(lines)-1]

	for _, line := range lines[:len(lines)-1] {
		b.lines = append(b.lines, line)
		for ch := range b.subs {
			select {
			case ch <- line:
			default: // slow subscriber, drop rather than block the deploy
			}
		}
	}
	return len(p), nil
}

// snapshotAndSubscribe returns everything logged so far plus a channel
// for lines that arrive later. The channel is closed semantics-free;
// callers watch b.done to know the deploy ended.
func (b *deployLogBuf) snapshotAndSubscribe() ([]string, chan string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan string, 256)
	b.subs[ch] = struct{}{}
	return append([]string(nil), b.lines...), ch
}

func (b *deployLogBuf) unsubscribe(ch chan string) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
}

func (b *deployLogBuf) text() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := strings.Join(b.lines, "\n")
	if b.partial != "" {
		if out != "" {
			out += "\n"
		}
		out += b.partial
	}
	return out
}

/*
DeploymentLogs returns the captured log of one deployment. While the
deploy runs it serves the live buffer; afterwards, the stored text.
*/
func (h *AppsHandler) DeploymentLogs(w http.ResponseWriter, r *http.Request) {
	app, err := db.GetApp(h.database, chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}

	depID := chi.URLParam(r, "depID")
	if b := h.deployLogs.get(depID); b != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"lines":   strings.Split(b.text(), "\n"),
			"running": true,
		})
		return
	}

	dep, err := db.GetDeployment(h.database, app.ID, depID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "deployment not found"})
		return
	}

	lines := []string{}
	if dep.Log != "" {
		lines = strings.Split(dep.Log, "\n")
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"lines":   lines,
		"running": dep.Status == "running",
	})
}

/*
DeploymentLogsStream is the SSE variant: it replays what has already
happened, then follows the deploy until it finishes. For an already
finished deployment it sends the stored log and ends the stream, so
clients can treat both cases the same way.
*/
func (h *AppsHandler) DeploymentLogsStream(w http.ResponseWriter, r *http.Request) {
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

	depID := chi.URLParam(r, "depID")
	b := h.deployLogs.get(depID)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	if b == nil {
		dep, err := db.GetDeployment(h.database, app.ID, depID)
		if err == nil && dep.Log != "" {
			for _, line := range strings.Split(dep.Log, "\n") {
				sendEvent(w, line)
			}
		}
		sendDone(w)
		flusher.Flush()
		return
	}

	replay, ch := b.snapshotAndSubscribe()
	defer b.unsubscribe(ch)

	for _, line := range replay {
		sendEvent(w, line)
	}
	flusher.Flush()

	heartbeat := time.NewTicker(streamHeartbeat)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-b.done:
			// Drain lines that raced the close, then finish.
			for {
				select {
				case line := <-ch:
					sendEvent(w, line)
				default:
					sendDone(w)
					flusher.Flush()
					return
				}
			}
		case line := <-ch:
			sendEvent(w, line)
			flusher.Flush()
		case <-heartbeat.C:
			w.Write([]byte(": ping\n\n"))
			flusher.Flush()
		}
	}
}

// sendDone signals end-of-deploy with a named SSE event, letting the
// client close the connection instead of retrying.
func sendDone(w http.ResponseWriter) {
	w.Write([]byte("event: done\ndata: \n\n"))
}
