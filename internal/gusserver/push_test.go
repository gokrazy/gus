package gusserver

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func dummyZip(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	f, err := w.Create("hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("hello world 👋")); err != nil {
		t.Fatal(err)
	}

	// Make sure to check the error on Close.
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func TestPush(t *testing.T) {
	// The push API does not access or modify any database state, so we only
	// test it with sqlite.
	t.Run("sqlite", func(t *testing.T) {
		_, mux, err := newServer("sqlite", ":memory:", &config{
			imageDir: t.TempDir(),
		})
		if err != nil {
			t.Fatal(err)
		}

		testsrv := httptest.NewServer(mux)
		client := testsrv.Client()
		zipb := dummyZip(t)
		req, err := http.NewRequest("PUT", testsrv.URL+"/api/v1/push", bytes.NewReader(zipb))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := resp.StatusCode, http.StatusOK; got != want {
			t.Fatalf("unexpected HTTP status code: got %v, want %v", resp.Status, want)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("reading response: %v", err)
		}
		var pr pushResponse
		if err := json.Unmarshal(body, &pr); err != nil {
			t.Fatalf("decoding JSON response: %v", err)
		}
		if pr.DownloadLink == "" {
			t.Fatalf("push response unexpectedly contains empty download_link")
		}

		// ensure we can download the same content via the download link
		req, err = http.NewRequest("GET", testsrv.URL+pr.DownloadLink, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err = client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := resp.StatusCode, http.StatusOK; got != want {
			t.Fatalf("unexpected HTTP status code: got %v, want %v", resp.Status, want)
		}
		if got, want := resp.Header.Get("Content-Type"), "application/zip"; got != want {
			t.Fatalf("unexpected HTTP Content-Type: got %q, want %q", got, want)
		}
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("reading response: %v", err)
		}
		if !bytes.Equal(zipb, body) {
			t.Fatalf("downloaded contents do not match pushed contents")
		}
	})
}
