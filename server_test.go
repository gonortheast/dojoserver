package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestCRUD(t *testing.T) {
	srv := httptest.NewServer(NewHandler())
	defer srv.Close()

	// Get with no entries.
	assertGet(t, srv.URL+"/server", `{}`)

	// Add one entry and check that we see it.
	t.Logf("add first entry")
	assertPostForm(t, srv.URL+"/server/", url.Values{
		"token": {tokens[4]},
		"url":   {"0.1.2.3:3294"},
	})

	// Wait for the server to poll the address.
	// What's a better approach here?
	time.Sleep(100 * time.Millisecond)
	expectStatus4 := "error: cannot get URL: Get http://0.1.2.3:3294/message: dial tcp 0.1.2.3:3294: connect: invalid argument"
	assertGet(t, srv.URL+"/server", fmt.Sprintf(`{"4":{"URL":"http://0.1.2.3:3294/","Status":%q}}`, expectStatus4))
	assertGet(t, srv.URL+"/server/4", fmt.Sprintf(`{"URL":"http://0.1.2.3:3294/","Status":%q}`, expectStatus4))

	// Add another entry and check that we see both.
	t.Logf("add second entry")
	assertPostForm(t, srv.URL+"/server/", url.Values{
		"token": {tokens[11]},
		"url":   {"https://0.1.2.4"},
	})
	time.Sleep(100 * time.Millisecond)
	expectStatus11 := "error: cannot get URL: Get https://0.1.2.4/message: dial tcp 0.1.2.4:443: connect: invalid argument"
	assertGet(t, srv.URL+"/server", fmt.Sprintf(`{"11":{"URL":"https://0.1.2.4/","Status":%q},"4":{"URL":"http://0.1.2.3:3294/","Status":%q}}`, expectStatus11, expectStatus4))
	assertGet(t, srv.URL+"/server/11", fmt.Sprintf(`{"URL":"https://0.1.2.4/","Status":%q}`, expectStatus11))

	// Update an existing entry.
	t.Logf("update existing entry")
	assertPostForm(t, srv.URL+"/server/", url.Values{
		"token": {tokens[4]},
		"url":   {"0.1.2.6"},
	})
	// Status won't have been polled again.
	assertGet(t, srv.URL+"/server", fmt.Sprintf(`{"11":{"URL":"https://0.1.2.4/","Status":%q},"4":{"URL":"http://0.1.2.6/","Status":%q}}`, expectStatus11, expectStatus4))

	// Delete an entry.
	t.Logf("delete an entry")
	resp, err := http.DefaultClient.Do(newRequest("DELETE", srv.URL+"/server/11?token="+tokens[11], ""))
	if err != nil {
		t.Fatalf("cannot do request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status from DELETE: %v", resp.StatusCode)
	}
	assertGet(t, srv.URL+"/server", fmt.Sprintf(`{"4":{"URL":"http://0.1.2.6/","Status":%q}}`, expectStatus4))
}

func assertPostForm(t *testing.T, url string, form url.Values) {
	resp, err := http.DefaultClient.PostForm(url, form)
	if err != nil {
		t.Fatalf("cannot do POST request on %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		t.Fatalf("unexpected status from POST: %v; body %s", resp.Status, body)
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
		t.Fatalf("unexpected data from %s\n\tgot %s\n\twant %s", url, data, expectContent)
	}
}
