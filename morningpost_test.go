package morningpost_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/thiagonache/morningpost"
)

func newServerWithContentTypeAndBodyResponse(t *testing.T, contentType string, filePath string) *httptest.Server {
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", contentType)
		w.Write(data)
	}))
	t.Cleanup(func() { ts.Close() })
	return ts
}

func TestServeHTTP_ReturnsNoContentGivenGetAtVisitWithURLVisited(t *testing.T) {
	t.Parallel()
	want := http.StatusNoContent
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/visit/https:%2F%2Fsite.url%2Fnews-xyz", nil)
	m := morningpost.New(&fakeStore{
		visited: map[string]bool{"https://site.url/news-xyz": true},
	})
	m.ServeHTTP(resp, req)
	got := resp.Code
	if want != got {
		t.Fatalf("want response status code %d, got %d", want, got)
	}
}

func TestServeHTTP_ReturnsNotFoundGivenGetAtVisitWithURLNotVisited(t *testing.T) {
	t.Parallel()
	want := http.StatusNotFound
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/visit/https:%2F%2Fsite.url%2Fnews-xyz", nil)
	m := morningpost.New(&fakeStore{
		visited: map[string]bool{},
	})
	m.ServeHTTP(resp, req)
	got := resp.Code
	if want != got {
		t.Fatalf("want response status code %d, got %d", want, got)
	}
}

func TestServeHTTP_ReturnsNotAllowedGivenRequestWithUnexpectedMethodAtVisit(t *testing.T) {
	t.Parallel()
	want := http.StatusMethodNotAllowed
	resp := httptest.NewRecorder()
	req := httptest.NewRequest("bogus", "/visit/", nil)
	m := morningpost.New(&fakeStore{})
	m.ServeHTTP(resp, req)
	got := resp.Code
	if want != got {
		t.Fatalf("want response status code %d, got %d", want, got)
	}
}

func TestServeHTTP_ReturnsOKGivenHeadAtVisit(t *testing.T) {
	t.Parallel()
	want := http.StatusOK
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodHead, "/visit/", nil)
	m := morningpost.New(&fakeStore{})
	m.ServeHTTP(resp, req)
	got := resp.Code
	if want != got {
		t.Fatalf("want response status code %d, got %d", want, got)
	}
}

func TestServeHTTP_ReturnsNoContentGivenPostAtVisit(t *testing.T) {
	t.Parallel()
	want := http.StatusNoContent
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/visit/https:%2F%2Fsite.url%2Fnews-xyz", nil)
	m := morningpost.New(&fakeStore{})
	m.ServeHTTP(resp, req)
	got := resp.Code
	if want != got {
		t.Fatalf("want resp status code %d, got %d", want, got)
	}
}

func TestServeHTTP_CallsRecordVisitWithProperURLGivenPostAtVisit(t *testing.T) {
	t.Parallel()
	want := "https://site.url/news-xyz"
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/visit/https:%2F%2Fsite.url%2Fnews-xyz", nil)
	store := &fakeStore{
		callRecordVisit: []string{},
	}
	m := morningpost.New(store)
	m.ServeHTTP(resp, req)
	if len(store.callRecordVisit) == 0 {
		t.Fatal("RecordVisit not called")
	}
	got := store.callRecordVisit[0]
	if want != got {
		t.Fatalf("want RecordVisit called with %q, got %q", want, got)
	}
}

func TestServeHTTP_ReturnsNoContentGivenGetAtFeedWithExistentFeed(t *testing.T) {
	t.Parallel()
	want := http.StatusNoContent
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/feed/https:%2F%2Ffeed.url%2Frss", nil)
	m := morningpost.New(&fakeStore{
		feeds: map[string]bool{"https://feed.url/rss": true},
	})
	m.ServeHTTP(resp, req)
	got := resp.Code
	if want != got {
		t.Fatalf("want response status code %d, got %d", want, got)
	}
}

func TestServeHTTP_ReturnsNotFoundGivenGetAtFeedWithNonExistentFeed(t *testing.T) {
	t.Parallel()
	want := http.StatusNotFound
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/feed/https:%2F%2Ffeed.url%2Frss", nil)
	m := morningpost.New(&fakeStore{})
	m.ServeHTTP(resp, req)
	got := resp.Code
	if want != got {
		t.Fatalf("want response status code %d, got %d", want, got)
	}
}

func TestServeHTTP_ReturnsNoContentGivenPostAtFeed(t *testing.T) {
	t.Parallel()
	want := http.StatusNoContent
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/feed/https:%2F%2Ffeed.url%2Frss", nil)
	m := morningpost.New(&fakeStore{})
	m.ServeHTTP(resp, req)
	got := resp.Code
	if want != got {
		t.Fatalf("want resp status code %d, got %d", want, got)
	}
}

func TestServeHTTP_ReturnsOKGivenHeadAtFeed(t *testing.T) {
	t.Parallel()
	want := http.StatusOK
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodHead, "/feed/", nil)
	m := morningpost.New(&fakeStore{})
	m.ServeHTTP(resp, req)
	got := resp.Code
	if want != got {
		t.Fatalf("want resp status code %d, got %d", want, got)
	}
}

func TestServeHTTP_CallsAddFeedWithProperURLGivenPostAtFeed(t *testing.T) {
	t.Parallel()
	tsRSSFeed := newServerWithContentTypeAndBodyResponse(t, "application/rss+xml", "testdata/rss.xml")
	want := tsRSSFeed.URL + "/rss"
	resp := httptest.NewRecorder()
	feedURLEscaped := url.PathEscape(tsRSSFeed.URL + "/rss")
	req := httptest.NewRequest(http.MethodPost, "/feed/"+feedURLEscaped, nil)
	store := &fakeStore{
		callAddFeed: []string{},
	}
	m := morningpost.New(store)
	m.ServeHTTP(resp, req)
	if len(store.callAddFeed) == 0 {
		t.Fatal("AddFeed not called")
	}
	got := store.callAddFeed[0]
	if want != got {
		t.Fatalf("want AddFeed called with %q, got %q", want, got)
	}
}

func TestServeHTTP_ReturnsNoContentGivenDeleteAtFeed(t *testing.T) {
	t.Parallel()
	want := http.StatusNoContent
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/feed/https:%2F%2Ffeed.url%2Frss", nil)
	m := morningpost.New(&fakeStore{})
	m.ServeHTTP(resp, req)
	got := resp.Code
	if want != got {
		t.Fatalf("want resp status code %d, got %d", want, got)
	}
}

func TestServeHTTP_CallsDeleteFeedWithProperURLGivenDeleteAtFeed(t *testing.T) {
	t.Parallel()
	want := "https://site.url/news-xyz"
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/feed/https:%2F%2Fsite.url%2Fnews-xyz", nil)
	store := &fakeStore{
		callDeleteFeed: []string{},
	}
	m := morningpost.New(store)
	m.ServeHTTP(resp, req)
	if len(store.callDeleteFeed) == 0 {
		t.Fatal("DeleteFeed not called")
	}
	got := store.callDeleteFeed[0]
	if want != got {
		t.Fatalf("want DeleteFeed called with %q, got %q", want, got)
	}
}

func TestServeHTTP_ReturnsNotAllowedGivenRequestWithUnexpectedMethodAtFeed(t *testing.T) {
	t.Parallel()
	want := http.StatusMethodNotAllowed
	resp := httptest.NewRecorder()
	req := httptest.NewRequest("bogus", "/feed/", nil)
	m := morningpost.New(&fakeStore{})
	m.ServeHTTP(resp, req)
	got := resp.Code
	if want != got {
		t.Fatalf("want response status code %d, got %d", want, got)
	}
}

func TestServeHTTP_ReturnsNotFoundGivenRequestAtUnkownRoute(t *testing.T) {
	t.Parallel()
	want := http.StatusNotFound
	resp := httptest.NewRecorder()
	req := httptest.NewRequest("", "/bogus", nil)
	server := morningpost.New(&fakeStore{})
	server.ServeHTTP(resp, req)
	got := resp.Code
	if want != got {
		t.Fatalf("want resp status code %d, got %d", want, got)
	}
}

func TestServeHTTP_ReturnsOKGivenGetAtFeedTableRows(t *testing.T) {
	t.Parallel()
	want := http.StatusOK
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/feed/table/rows", nil)
	m := morningpost.New(&fakeStore{})
	m.ServeHTTP(resp, req)
	got := resp.Code
	if want != got {
		t.Fatalf("want response status code %d, got %d", want, got)
	}
}

func TestServeHTTP_ReturnsNotAllowedGivenRequestWithUnexpectedMethodAtFeedTableRows(t *testing.T) {
	t.Parallel()
	want := http.StatusMethodNotAllowed
	resp := httptest.NewRecorder()
	req := httptest.NewRequest("bogus", "/feed/table/rows", nil)
	m := morningpost.New(&fakeStore{})
	m.ServeHTTP(resp, req)
	got := resp.Code
	if want != got {
		t.Fatalf("want response status code %d, got %d", want, got)
	}
}

func TestServeHTTP_ReturnsExpectedContentGivenGetAtFeedTableRowsAndPopulatedStore(t *testing.T) {
	t.Parallel()
	want := `<tr>
  <th scope="row">https://feed.url/rss</th>
  <td>
    <button
      class="btn btn-danger"
      hx-delete="/feeds/https:%2F%2Ffeed.url%2Frss"
      hx-confirm="Please, confirm you want to delete this feed."
      hx-target="closest tr"
      hx-swap="outerHTML swap:1s"
    >
      Delete
    </button>
  </td>
</tr>`
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/feed/table/rows", nil)
	m := morningpost.New(&fakeStore{
		feeds: map[string]bool{
			"https://feed.url/rss": true,
		},
	})
	m.ServeHTTP(resp, req)
	got := resp.Body.String()
	got = strings.Join(strings.FieldsFunc(strings.ReplaceAll(got, "\r\n", "\n"), func(r rune) bool {
		return r == '\n' || r == '\r'
	}), "\n")
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestFindFeeds_ReturnsExpectedFeedsGivenApplicationRSSXMLContentType(t *testing.T) {
	t.Parallel()
	ts := newServerWithContentTypeAndBodyResponse(t, "application/rss+xml", "testdata/rss.xml")
	want := []morningpost.Feed{
		{
			URL:  ts.URL,
			Type: morningpost.FeedTypeRSS,
		},
	}
	m := morningpost.New(&fakeStore{})
	got := m.FindFeeds(ts.URL)
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestFindFeeds_ReturnsExpectedFeedsGivenApplicationXMLContentTypeAndRSSData(t *testing.T) {
	t.Parallel()
	ts := newServerWithContentTypeAndBodyResponse(t, "application/xml", "testdata/rss.xml")
	want := []morningpost.Feed{
		{
			URL:  ts.URL,
			Type: morningpost.FeedTypeRSS,
		},
	}
	m := morningpost.New(&fakeStore{})
	got := m.FindFeeds(ts.URL)
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestFindFeeds_ReturnsExpectedFeedsGivenTextXMLContentTypeAndRSSData(t *testing.T) {
	t.Parallel()
	ts := newServerWithContentTypeAndBodyResponse(t, "text/xml", "testdata/rss.xml")
	want := []morningpost.Feed{
		{
			URL:  ts.URL,
			Type: morningpost.FeedTypeRSS,
		},
	}
	m := morningpost.New(&fakeStore{})
	got := m.FindFeeds(ts.URL)
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestFindFeeds_ReturnsExpectedFeedsGivenTextXMLContentTypeAndRDFData(t *testing.T) {
	t.Parallel()
	ts := newServerWithContentTypeAndBodyResponse(t, "text/xml", "testdata/rdf.xml")
	want := []morningpost.Feed{
		{
			URL:  ts.URL,
			Type: morningpost.FeedTypeRDF,
		},
	}
	m := morningpost.New(&fakeStore{})
	got := m.FindFeeds(ts.URL)
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestFindFeeds_ReturnsExpectedFeedsGivenAtomApplicationContentType(t *testing.T) {
	t.Parallel()
	ts := newServerWithContentTypeAndBodyResponse(t, "application/atom+xml", "testdata/atom.xml")
	want := []morningpost.Feed{
		{
			URL:  ts.URL,
			Type: "Atom",
		},
	}
	m := morningpost.New(&fakeStore{})
	got := m.FindFeeds(ts.URL)
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestFindFeeds_ReturnsExpectedFeedsGivenHTMLPageWithFeedsInFullLinkFormat(t *testing.T) {
	t.Parallel()
	want := []morningpost.Feed{
		{
			URL:  "http://example.com/rss",
			Type: morningpost.FeedTypeRSS,
		},
		{
			URL:  "http://example.com/atom",
			Type: morningpost.FeedTypeAtom,
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		case "/":
			w.Header().Set("content-type", "text/html")
			w.Write([]byte(`<link type="application/rss+xml" title="RSS Unit Test" href="http://example.com/rss" />
	                        <link type="application/atom+xml" title="Atom Unit Test" href="http://example.com/atom" />`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	m := morningpost.New(&fakeStore{})
	got := m.FindFeeds(ts.URL)
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestFindFeeds_ReturnsExpectedFeedsGivenHTMLPageWithFeedInRelativeLinkFormat(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		case "/":
			w.Header().Set("content-type", "text/html")
			w.Write([]byte(`<link type="application/rss+xml" title="RSS Unit Test" href="rss" />`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	want := []morningpost.Feed{
		{
			URL:  ts.URL + "/rss",
			Type: morningpost.FeedTypeRSS,
		},
	}
	m := morningpost.New(&fakeStore{})
	got := m.FindFeeds(ts.URL)
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestFeedFinds_SetsHeadersOnHTTPRequest(t *testing.T) {
	t.Parallel()
	wantHeaders := map[string]string{
		"user-agent": "MorningPost/0.1",
		"accept":     "*/*",
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for header, want := range wantHeaders {
			got := r.Header.Get(header)
			if want != got {
				t.Errorf("want value %q, got %q for header %q", want, got, header)
			}
		}
		w.Header().Set("content-type", "application/rss+xml")
		w.Write([]byte(`<rss></rss>`))
	}))
	defer ts.Close()
	m := morningpost.New(&fakeStore{})
	m.FindFeeds(ts.URL)
}

func TestParseLinkTags_ReturnsFeedEndpointGivenHTMLPageWithRSSFeedInBodyElement(t *testing.T) {
	t.Parallel()
	want := []morningpost.Feed{
		{
			URL:  "https://bitfieldconsulting.com/golang?format=rss",
			Type: morningpost.FeedTypeRSS,
		},
	}
	got, err := morningpost.ParseLinkTags(strings.NewReader(`<a href="https://bitfieldconsulting.com/golang?format=rss" title="Go RSS" class="social-rss">Go RSS</a>`), "")
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestParseLinkTags_ReturnsFeedEndpointGivenHTMLPageWithRSSFeedInLinkElement(t *testing.T) {
	t.Parallel()
	want := []morningpost.Feed{
		{
			URL:  "http://fake.url/rss",
			Type: morningpost.FeedTypeRSS,
		},
	}
	got, err := morningpost.ParseLinkTags(strings.NewReader(`<link type="application/rss+xml" title="Unit Test" href="http://fake.url/rss" />`), "")
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestParseLinkTags_ReturnsFeedEndpointGivenHTMLPageWithAtomFeedInLinkElement(t *testing.T) {
	t.Parallel()
	want := []morningpost.Feed{
		{
			URL:  "http://fake.url/feed/",
			Type: morningpost.FeedTypeAtom,
		},
	}
	got, err := morningpost.ParseLinkTags(strings.NewReader(`<link type="application/atom+xml" title="Unit Test" href="http://fake.url/feed/" />`), "")
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestParseFeedType_ReturnsRSSTypeGivenRSSTag(t *testing.T) {
	t.Parallel()
	want := morningpost.FeedTypeRSS
	got, err := morningpost.ParseFeedType(strings.NewReader(`<?xml version="1.0"?>
<rss version="2.0"></rss>`))
	if err != nil {
		t.Fatal(err)
	}
	if want != got {
		t.Fatalf("want XMLName %q, got %q", want, got)
	}
}

func TestParseFeedType_ReturnsAtomTypeGivenFeedTag(t *testing.T) {
	t.Parallel()
	want := morningpost.FeedTypeAtom
	got, err := morningpost.ParseFeedType(strings.NewReader(`<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom"></feed>`))
	if err != nil {
		t.Fatal(err)
	}
	if want != got {
		t.Fatalf("want XMLName %q, got %q", want, got)
	}
}

func TestParseFeedType_ErrorsGivenUnexpectedTag(t *testing.T) {
	t.Parallel()
	_, err := morningpost.ParseFeedType(strings.NewReader(`<bogus></bogus>`))
	if err == nil {
		t.Fatal("want error but got nil")
	}
}

func TestNew_SetsHTTPTimeoutByDefault(t *testing.T) {
	t.Parallel()
	want := morningpost.DefaultHTTPTimeout
	m := morningpost.New(&fakeStore{})
	got := m.Client.Timeout
	if want != got {
		t.Fatalf("want timeout %v, got %v", want, got)
	}
}
