package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

func main() {
	http.ListenAndServe(":8080", NewHandler())
}

// NewHandler returns a new dojo server handler.
func NewHandler() http.Handler {
	h := &handler{
		addresses: make(map[string]*address),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/address/", h.serveAddresses)
	return mux
}

type address struct {
	Address string
}

type handler struct {
	mu        sync.Mutex
	addresses map[string]*address
}

// serveAddress serves the top level /address endpoint.
func (h *handler) serveAddress(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r.Method != "GET" {
		http.Error(w, "only GET allowed", http.StatusMethodNotAllowed)
	}
	writeJSON(w, h.addresses)
}

// serveAddresses servers individual address endpoints under /address
func (h *handler) serveAddresses(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	log.Printf("%s %s", r.Method, r.URL)
	name := strings.TrimPrefix(r.URL.Path, "/address/")
	if strings.Contains(name, "/") {
		http.Error(w, "bad address path", http.StatusBadRequest)
		return
	}
	if name == "" && r.Method != "GET" {
		http.Error(w, "only GET allowed", http.StatusMethodNotAllowed)
		return
	}
	switch r.Method {
	case "GET":
		if name == "" {
			// GET of the top level endpoint returns all addresses.
			writeJSON(w, h.addresses)
			return
		}
		addr := h.addresses[name]
		if addr == nil {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, addr)
	case "PUT":
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("cannot read body: %v", err), http.StatusBadRequest)
			return
		}
		var addr address
		if err := json.Unmarshal(data, &addr); err != nil {
			http.Error(w, fmt.Sprintf("cannot unmarshal body: %v", err), http.StatusBadRequest)
			return
		}
		u, err := url.Parse(addr.Address)
		if err != nil {
			http.Error(w, fmt.Sprintf("bad address URL: %v", err), http.StatusBadRequest)
			return
		}
		if !strings.HasSuffix(u.Path, "/") {
			u.Path += "/"
		}
		if u.Scheme == "" {
			u.Scheme = "http"
		}
		addr.Address = u.String()
		h.addresses[name] = &addr
	case "DELETE":
		delete(h.addresses, name)
	default:
		http.Error(w, "only GET, PUT and DELETE allowed", http.StatusMethodNotAllowed)
	}
}

func writeJSON(w http.ResponseWriter, x interface{}) {
	data, err := json.Marshal(x)
	if err != nil {
		http.Error(w, fmt.Sprintf("cannot marshal addresses: %v"), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
