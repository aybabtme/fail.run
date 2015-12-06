package svc

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aybabtme/log"
)

const idLen = 32

var (
	MaxPageLifetime = 30 * time.Second

	newPath  = "/new"
	sinkPath = "/s/"
)

type handler struct {
	files http.Handler

	random *rand.Rand

	sinksl sync.Mutex
	sinks  map[string]http.Handler
}

// New creates a service handler.
func New(r *rand.Rand, filepath string) http.Handler {
	return &handler{
		files:  http.FileServer(http.Dir(filepath)),
		random: r,
		sinks:  make(map[string]http.Handler),
	}
}

func (svc *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == newPath {
		r.URL.Path = sinkPath + genID(svc.random)
		http.Redirect(w, r, r.URL.String(), http.StatusSeeOther)
		return
	}
	if strings.HasPrefix(r.URL.Path, sinkPath) {
		// don't want to store custom IDs that are too long
		if len(r.URL.Path) > idLen {
			r.URL.Path = r.URL.Path[:idLen]
			http.Redirect(w, r, r.URL.String(), http.StatusSeeOther)
			return
		}

		handler := svc.getOrCreate(r.URL.Path, MaxPageLifetime)
		handler.ServeHTTP(w, r)
		return
	}
	svc.files.ServeHTTP(w, r)
}

func (svc *handler) getOrCreate(id string, dieIn time.Duration) http.Handler {
	if len(id) > idLen {
		panic(len(id))
	}
	svc.sinksl.Lock()
	hdl, ok := svc.sinks[id]
	if !ok {
		hdl = startHandler(id, dieIn, func() {
			svc.sinksl.Lock()
			delete(svc.sinks, id)
			svc.sinksl.Unlock()
		})
		svc.sinks[id] = hdl
	}
	svc.sinksl.Unlock()
	return hdl
}

type pageHandler struct {
	id      string
	refresh chan struct{}
	mu      sync.Mutex
	lastRPS []measure
}

type measure struct {
	UTCUnix int64 `json:"utc_unix"`
	RPS     int   `json:"rps"`
}

func startHandler(id string, dieIn time.Duration, die func()) http.Handler {
	maxHistory := int(dieIn.Seconds())
	hdl := &pageHandler{
		id:      id,
		refresh: make(chan struct{}),
		lastRPS: append([]measure{{
			UTCUnix: time.Now().Truncate(time.Second).UTC().Unix(),
		}}, make([]measure, 0, maxHistory)...),
	}

	go func() {
		deadline := time.NewTimer(dieIn)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-deadline.C:
				die()
				return
			case <-hdl.refresh:
				deadline.Reset(dieIn)
			case now := <-ticker.C:
				hdl.mu.Lock()
				hdl.lastRPS = append(hdl.lastRPS, measure{
					UTCUnix: now.Truncate(time.Second).UTC().Unix(),
				})
				if len(hdl.lastRPS) > maxHistory {
					hdl.lastRPS = hdl.lastRPS[1:]
				}
				hdl.mu.Unlock()
			}
		}
	}()
	return hdl
}

func (h *pageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	select {
	case h.refresh <- struct{}{}:
	default:
	}

	buf := bytes.NewBuffer(nil)
	h.mu.Lock()
	h.lastRPS[len(h.lastRPS)-1].RPS++
	if err := json.NewEncoder(buf).Encode(h.lastRPS); err != nil {
		log.Err(err).Error("can't encode RPS")
		return
	}
	h.mu.Unlock()
	w.WriteHeader(http.StatusOK)
	buf.WriteTo(w)
}

func genID(r *rand.Rand) string {
	const alphabet = "0123456789abcdefghijklmnopqrstuvvxyzABCDEFGHIJKLMNOPQRSTUVVXYZ"
	buf := make([]byte, 0, idLen)
	for len(buf) < idLen {
		buf = append(buf, alphabet[r.Intn(len(alphabet))])
	}
	return string(buf)
}
