package morningpost_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/thiagonache/morningpost"
)

func newRSSFeedServer(t testing.TB) *httptest.Server {
	data, err := os.ReadFile("testdata/rss.xml")
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		case "/rss":
			w.Header().Set("content-type", "application/rss+xml")
			w.Write(data)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(func() { ts.Close() })
	return ts
}

func TestIntegrationVisit(t *testing.T) {
	t.Parallel()
	want := http.StatusNoContent
	store := &morningpost.MemoryStore{}
	m := morningpost.New(store)
	m.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/visit/https:%2F%2Ffeed.url%2Frss", nil))
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/visit/https:%2F%2Ffeed.url%2Frss", nil)
	m.ServeHTTP(resp, req)
	got := resp.Code
	if want != got {
		t.Fatalf("want response code %d, got %d", want, got)
	}
}

func TestIntegrationFeed(t *testing.T) {
	t.Parallel()
	tsRSSFeed := newRSSFeedServer(t)
	feedURL := tsRSSFeed.URL + "/rss"
	feedURLEscaped := url.PathEscape(feedURL)
	feedAPIEndpoint := "/feed/" + feedURLEscaped
	store := &morningpost.MemoryStore{}
	m := morningpost.New(store)
	m.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, feedAPIEndpoint, nil))
	resp := httptest.NewRecorder()
	m.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, feedAPIEndpoint, nil))
	if http.StatusNoContent != resp.Code {
		t.Fatalf("want feed %q to exist but it does not", feedURL)
	}
	m.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodDelete, feedAPIEndpoint, nil))
	resp = httptest.NewRecorder()
	m.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, feedAPIEndpoint, nil))
	if http.StatusNotFound != resp.Code {
		t.Fatalf("feed %q should not exist but it does", feedURL)
	}
}
