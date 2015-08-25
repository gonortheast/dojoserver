/*
The dojo server implements a simple address-exchange server.  A POST
request  to /server?token=$teamtoken&url=$url will create a server
instance that will be polled by the dojo server to find its message.

The address and status of a server can be retrieved with a GET request
to /server/$teamnumber, where $teamnumber is the number of the team,
derived from the team token. The response is JSON encoded in the form:

	struct {
		URL     string
		Status  string
	}

where URL is the address of the server and Status is "ok" if the server
is up and running and has a message and holds an error message otherwise.

All the servers can be retrieved with a GET request to /server, JSON
encoded in the form:

	map[string] struct {
		URL     string
		Status  string
	}

where each entry in the map holds a server entry, keyed by its team
number.

A server entry can be deleted by sending a DELETE request to
/server/$teamnumber?token=$teamtoken (the correct token for the team
must be provided).
*/
package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

func main() {
	http.ListenAndServe(":8080", NewHandler())
}

func NewHandler() http.Handler {
	h := &handler{
		servers: make(map[int]*server),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/server", h.serveServers)
	mux.HandleFunc("/server/", h.serveServers)
	return mux
}

type server struct {
	URL     string
	Status  string
	Message string `json:",omitempty"`
}

type handler struct {
	mu      sync.Mutex
	servers map[int]*server
}

// serveServer serves the top level /server endpoint.
func (h *handler) serveServer(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r.Method != "GET" {
		http.Error(w, "only GET allowed", http.StatusMethodNotAllowed)
	}
	writeJSON(w, h.servers)
}

// serveServers servers individual server endpoints under /server
func (h *handler) serveServers(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s", r.Method, r.URL)
	h.mu.Lock()
	defer h.mu.Unlock()
	name := strings.TrimPrefix(r.URL.Path, "/server")
	name = strings.TrimPrefix(name, "/")

	if strings.Contains(name, "/") {
		http.Error(w, "bad server path", http.StatusBadRequest)
		return
	}
	team := -1
	if name != "" {
		n, err := strconv.Atoi(name)
		if err != nil || n < 0 {
			http.Error(w, "invalid server path", http.StatusBadRequest)
		}
		team = n
	}
	switch r.Method {
	case "GET":
		if team == -1 {
			h.serveGetAllServers(w, r)
		} else {
			h.serveGetServer(team, w, r)
		}
	case "POST":
		if team != -1 {
			http.Error(w, "you can only POST to /server", http.StatusMethodNotAllowed)
			return
		}
		h.servePostServer(w, r)
	case "DELETE":
		if team == -1 {
			http.Error(w, "you cannot delete /server", http.StatusMethodNotAllowed)
			return
		}
		h.serveDeleteServer(team, w, r)
	default:
		http.Error(w, "only GET, POST and DELETE allowed", http.StatusMethodNotAllowed)
	}
}

func (h *handler) serveGetAllServers(w http.ResponseWriter, r *http.Request) {
	resp := make(map[string]server)
	for team, srv := range h.servers {
		srv1 := *srv
		// Clear out message so that clients can't cheat.
		srv1.Message = ""
		resp[fmt.Sprint(team)] = srv1
	}
	writeJSON(w, resp)
}

func (h *handler) serveGetServer(team int, w http.ResponseWriter, r *http.Request) {
	addr := h.servers[team]
	if addr == nil {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, addr)
}

func (h *handler) servePostServer(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, fmt.Sprintf("bad form: %v", err), http.StatusBadRequest)
		return
	}
	tok := r.Form.Get("token")
	if tok == "" {
		http.Error(w, "no token found in request", http.StatusBadRequest)
		return
	}
	urlStr := r.Form.Get("url")
	if urlStr == "" {
		http.Error(w, "no address found in request", http.StatusBadRequest)
		return
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("bad server URL: %v", err), http.StatusBadRequest)
		return
	}
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	if u.Scheme == "" {
		u.Scheme = "http"
	}
	team := teamNumber(tok)
	if team < 0 {
		http.Error(w, fmt.Sprintf("unknown team token %q", tok), http.StatusBadRequest)
		return
	}
	srv := h.servers[team]
	if srv == nil {
		h.servers[team] = &server{
			URL: u.String(),
		}
		go h.monitor(team)
	} else {
		h.servers[team].URL = u.String()
	}
}

func (h *handler) serveDeleteServer(team int, w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	if teamNumber(r.Form.Get("token")) != team {
		http.Error(w, "invalid token parameter", http.StatusBadRequest)
	}
	delete(h.servers, team)
}

func writeJSON(w http.ResponseWriter, x interface{}) {
	data, err := json.Marshal(x)
	if err != nil {
		http.Error(w, fmt.Sprintf("cannot marshal servers: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (h *handler) monitor(team int) {
	for {
		msg, err := h.pollWithTimeout(team, time.Second)
		if err != nil {
			h.setTeamStatus(team, fmt.Sprintf("error: %v", err))
		} else {
			h.setTeamStatus(team, "ok")
			h.setTeamMesssage(team, msg)
		}
		time.Sleep(time.Second)
	}
}

func (h *handler) urlForTeam(team int) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.servers[team].URL
}

func (h *handler) setTeamStatus(team int, status string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.servers[team].Status = status
}

func (h *handler) setTeamMesssage(team int, msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.servers[team].Message = msg
}

func (h *handler) pollWithTimeout(team int, timeout time.Duration) (string, error) {
	type pollResp struct {
		msg string
		err error
	}
	c := make(chan pollResp, 1)
	go func() {
		msg, err := h.poll(team)
		c <- pollResp{msg, err}
	}()
	select {
	case resp := <-c:
		return resp.msg, resp.err
	case <-time.After(time.Second):
		return "", fmt.Errorf("timed out trying to connect to server")
	}
}

func (h *handler) poll(team int) (string, error) {
	srvURL := h.urlForTeam(team)
	if srvURL == "" {
		return "", fmt.Errorf("no URL found for team")
	}
	return getBody(fmt.Sprintf("%smessage", srvURL))
}

func getBody(url string) (string, error) {
	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("cannot get URL: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("invalid status code %s from server", resp.Status)
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("cannot read message body: %v", err)
	}
	return string(data), nil
}

var secret = os.Getenv("DOJO_SECRET")

func teamTokens(n int) []string {
	tokens := make([]string, n)
	data := []byte(secret)
	for i := range tokens {
		tokData := md5.Sum(data)
		tokens[i] = fmt.Sprintf("%x", tokData)
		data = tokData[:]
	}
	return tokens
}

var tokens = teamTokens(20)

func teamNumber(s string) int {
	for i, t := range tokens {
		if s == t {
			return i
		}
	}
	return -1
}
