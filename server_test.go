package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCRUD(t *testing.T) {
	srv := httptest.NewServer(NewHandler())
	defer srv.Close()

	// Get with no entries.
	assertGet(t, srv.URL+"/address", `{}`)

	// Add one entry and check that we see it.
	assertPut(t, srv.URL+"/address/foo", `{"Address": "0.1.2.3:3294"}`)
	assertGet(t, srv.URL+"/address", `{"foo":{"Address":"http://0.1.2.3:3294/"}}`)
	assertGet(t, srv.URL+"/address/foo", `{"Address":"http://0.1.2.3:3294/"}`)

	// Add another entry and check that we see both.
	assertPut(t, srv.URL+"/address/bar", `{"Address": "https://0.1.2.4"}`)
	assertGet(t, srv.URL+"/address", `{"bar":{"Address":"https://0.1.2.4/"},"foo":{"Address":"http://0.1.2.3:3294/"}}`)
	assertGet(t, srv.URL+"/address/bar", `{"Address":"https://0.1.2.4/"}`)

	// Update an existing entry.
	assertPut(t, srv.URL+"/address/bar", `{"Address": "0.1.2.6"}`)
	assertGet(t, srv.URL+"/address", `{"bar":{"Address":"http://0.1.2.6/"},"foo":{"Address":"http://0.1.2.3:3294/"}}`)

	// Delete an entry.
	resp, err := http.DefaultClient.Do(newRequest("DELETE", srv.URL+"/address/bar", ""))
	if err != nil {
		t.Fatalf("cannot do request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status from DELETE: %v", resp.StatusCode)
	}
	assertGet(t, srv.URL+"/address", `{"foo":{"Address":"http://0.1.2.3:3294/"}}`)
}

func assertPut(t *testing.T, url string, bodyStr string) {
	resp, err := http.DefaultClient.Do(newRequest("PUT", url, bodyStr))
	if err != nil {
		t.Fatalf("cannot do PUT request on %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status from PUT: %v", resp.StatusCode)
	}
}

func newRequest(method, url, bodyStr string) *http.Request {
	var body io.Reader
	if bodyStr != "" {
		body = strings.NewReader(bodyStr)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		panic(fmt.Errorf("cannot make new request: %v", err))
	}
	return req
}

func assertGet(t *testing.T, url string, expectContent string) {
	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		t.Fatalf("GET %s failed: %v", url, err)
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("cannot read response body from %s: %v", url, err)
	}
	if string(data) != expectContent {
		t.Fatalf("unexpected data from %s; got %q want %q", url, data, expectContent)
	}
}
