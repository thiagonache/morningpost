package morningpost_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/thiagonache/morningpost"
	"golang.org/x/net/nettest"
)

func newServerWithContentTypeResponse(t *testing.T, contentType string) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", contentType)
	}))
	t.Cleanup(func() { ts.Close() })
	return ts
}

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

func newMorningPostWithFakeStoreAndNoOutput(t *testing.T) *morningpost.MorningPost {
	m, err := morningpost.New(
		fakeStore{},
		morningpost.WithStdout(io.Discard),
		morningpost.WithStderr(io.Discard),
	)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func generateNews(count int) []morningpost.News {
	allNews := []morningpost.News{}
	for x := 0; x < count; x++ {
		title := fmt.Sprintf("News #%d", x+1)
		URL := fmt.Sprintf("http://fake.url/news-%d", x+1)
		news := morningpost.News{
			Feed:  "Feed Unit test",
			Title: title,
			URL:   URL,
		}
		allNews = append(allNews, news)
	}
	return allNews
}

func removeEmptyLines(data []byte) []byte {
	var buf bytes.Buffer
	for _, b := range data {
		if b != '\n' {
			buf.WriteByte(b)
		} else if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] != '\n' {
			buf.WriteByte(b)
		}
	}
	return buf.Bytes()
}

func waitServerHealthCheck(t *testing.T, listenAddress string) {
	for x := 0; x < 10; x++ {
		resp, err := http.Get("http://" + listenAddress + "/healthz")
		if err != nil {
			time.Sleep(300 * time.Millisecond)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			time.Sleep(300 * time.Millisecond)
			continue
		}
		return
	}
	t.Fatalf("%q not responding to GET", listenAddress)
}

func TestNew_SetsDefaultNewsPageSizeByDefault(t *testing.T) {
	t.Parallel()
	want := morningpost.DefaultNewsPageSize
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	got := m.NewsPageSize
	if want != got {
		t.Fatalf("want ShowMaxNews %d but got %d", want, got)
	}
}

func TestNew_SetsDefaultListenAddressByDefault(t *testing.T) {
	t.Parallel()
	want := morningpost.DefaultListenAddress
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	got := m.ListenAddress
	if want != got {
		t.Fatalf("want DefaultListenAddress %q, got %q", want, got)
	}
}

func TestNew_SetsStderrByDefault(t *testing.T) {
	t.Parallel()
	want := os.Stderr
	m, err := morningpost.New(
		fakeStore{},
		morningpost.WithStdout(io.Discard),
	)
	if err != nil {
		t.Fatal(err)
	}
	got := m.Stderr
	if want != got {
		t.Fatalf("want stderr %+v, got %+v", want, got)
	}
}

func TestNew_SetsStdoutByDefault(t *testing.T) {
	t.Parallel()
	want := os.Stdout
	m, err := morningpost.New(
		fakeStore{},
		morningpost.WithStderr(io.Discard),
	)
	if err != nil {
		t.Fatal(err)
	}
	got := m.Stdout
	if want != got {
		t.Fatalf("want stdout %+v, got %+v", want, got)
	}
}

func TestNew_SetsHTTPTimeoutByDefault(t *testing.T) {
	t.Parallel()
	want := morningpost.DefaultHTTPTimeout
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	got := m.Client.Timeout
	if want != got {
		t.Fatalf("want timeout %v, got %v", want, got)
	}
}

func TestFromArgs_ErrorsIfUnkownFlag(t *testing.T) {
	t.Parallel()
	args := []string{"-asdfaewrawers", "8080"}
	_, err := morningpost.New(
		fakeStore{},
		morningpost.WithStderr(io.Discard),
		morningpost.FromArgs(args),
	)
	if err == nil {
		t.Fatal("want error but got nil")
	}
}

func TestFromArgs_pFlagSetListenPort(t *testing.T) {
	t.Parallel()
	want := "0.0.0.0:8080"
	args := []string{"-l", "0.0.0.0:8080"}
	m, err := morningpost.New(
		fakeStore{},
		morningpost.FromArgs(args),
	)
	if err != nil {
		t.Fatal(err)
	}
	got := m.ListenAddress
	if want != got {
		t.Errorf("wants -l flag to set listen address to %q, got %q", want, got)
	}
}

func TestFromArgs_SetsListenPortByDefaultToDefaultListenPort(t *testing.T) {
	t.Parallel()
	want := morningpost.DefaultListenAddress
	m, err := morningpost.New(
		fakeStore{},
		morningpost.FromArgs([]string{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	got := m.ListenAddress
	if want != got {
		t.Errorf("wants default listen address to %q, got %q", want, got)
	}
}

func TestWithStderr_ErrorsIfInputIsNil(t *testing.T) {
	t.Parallel()
	_, err := morningpost.New(
		fakeStore{},
		morningpost.WithStderr(nil),
	)
	if err == nil {
		t.Fatal("want error but got nil")
	}
}

func TestWithStderr_SetsStderrGivenWriter(t *testing.T) {
	t.Parallel()
	want := "this is a string\n"
	buf := &bytes.Buffer{}
	m, err := morningpost.New(
		fakeStore{},
		morningpost.WithStderr(buf),
	)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(m.Stderr, "this is a string")
	got := buf.String()
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestWithStdout_ErrorsIfInputIsNil(t *testing.T) {
	t.Parallel()
	_, err := morningpost.New(
		fakeStore{},
		morningpost.WithStdout(nil),
	)
	if err == nil {
		t.Fatal("want error but got nil")
	}
}

func TestWithStdout_SetsStdoutGivenWriter(t *testing.T) {
	t.Parallel()
	want := "this is a string\n"
	buf := &bytes.Buffer{}
	m, err := morningpost.New(
		fakeStore{},
		morningpost.WithStdout(buf),
	)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(m.Stdout, "this is a string")
	got := buf.String()
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestWithClient_ErrorsIfInputIsNil(t *testing.T) {
	t.Parallel()
	_, err := morningpost.New(
		fakeStore{},
		morningpost.WithClient(nil),
	)
	if err == nil {
		t.Fatal("want error but got nil")
	}
}

func TestWithClient_SetsDisableKeepAlivesGivenHTTPClientWithKeepAlivesDisabled(t *testing.T) {
	t.Parallel()
	want := true
	m, err := morningpost.New(
		fakeStore{},
		morningpost.WithClient(&http.Client{
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	got := m.Client.Transport.(*http.Transport).Clone().DisableKeepAlives
	if want != got {
		t.Fatalf("want DisableKeepAlives %t, got %t", want, got)
	}
}

func TestParseRSSResponse_ReturnsNewsGivenRSSFeedData(t *testing.T) {
	t.Parallel()
	want := []morningpost.News{
		{
			Feed:  "FeedForAll Sample Feed",
			Title: "RSS Solutions for Restaurants",
			URL:   "http://www.feedforall.com/restaurant.htm",
		},
		{
			Feed:  "FeedForAll Sample Feed",
			Title: "RSS Solutions for Schools and Colleges",
			URL:   "http://www.feedforall.com/schools.htm",
		},
	}
	file, err := os.Open("testdata/rss.xml")
	if err != nil {
		t.Fatalf("Cannot open file testdata/rss.xml: %+v", err)
	}
	got, err := morningpost.ParseRSSResponse(file)
	if err != nil {
		t.Fatalf("Cannot parse content: %+v", err)
	}
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestParseRSSResponse_ErrorsIfDataIsNotXML(t *testing.T) {
	t.Parallel()
	_, err := morningpost.ParseRSSResponse(strings.NewReader("{}"))
	if err == nil {
		t.Fatalf("want error but not found")
	}
}

func TestParseRDFResponse_ReturnsNewsGivenRDFData(t *testing.T) {
	t.Parallel()
	want := []morningpost.News{
		{
			Feed:  "Slashdot",
			Title: "Ask Slashdot:  What Was Your Longest-Lived PC?",
			URL:   "https://ask.slashdot.org/story/23/04/02/0058226/ask-slashdot-what-was-your-longest-lived-pc?utm_source=rss1.0mainlinkanon&utm_medium=feed",
		},
		{
			Feed:  "Slashdot",
			Title: "San Francisco Faces 'Doom Loop' from Office Workers Staying Home, Gutting Tax Base",
			URL:   "https://it.slashdot.org/story/23/04/01/2059224/san-francisco-faces-doom-loop-from-office-workers-staying-home-gutting-tax-base?utm_source=rss1.0mainlinkanon&utm_medium=feed",
		},
	}
	rdf, err := os.Open("testdata/rdf.xml")
	if err != nil {
		t.Fatalf("Cannot read file content: %+v", err)
	}
	got, err := morningpost.ParseRDFResponse(rdf)
	if err != nil {
		t.Fatalf("Cannot parse rdf content: %+v", err)
	}
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestParseRDFResponse_ErrorsIfDataIsNotXML(t *testing.T) {
	t.Parallel()
	_, err := morningpost.ParseRDFResponse(strings.NewReader("{}"))
	if err == nil {
		t.Fatalf("want error but not found")
	}
}

func TestHandleFeeds_AnswersMethodNotAllowedGivenRequestWithBogusMethod(t *testing.T) {
	t.Parallel()
	want := http.StatusMethodNotAllowed
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	ts := httptest.NewServer(http.HandlerFunc(m.HandleFeeds))
	defer ts.Close()
	req, err := http.NewRequest("bogus", ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	got := resp.StatusCode
	if want != got {
		t.Fatalf("want status code %d, got %d", want, got)
	}
}

func TestHandleFeeds_AnswersStatusOKGivenRequestWithMethodHead(t *testing.T) {
	t.Parallel()
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	ts := httptest.NewServer(http.HandlerFunc(m.HandleFeeds))
	defer ts.Close()
	resp, err := http.Head(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code %d", resp.StatusCode)
	}
}

func TestHandleFeeds_ReturnsExpectedStatusCodeGivenRequestWithMethodPostAndBody(t *testing.T) {
	t.Parallel()
	want := http.StatusOK
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	tsHandler := httptest.NewServer(http.HandlerFunc(m.HandleFeeds))
	defer tsHandler.Close()
	ts := newServerWithContentTypeAndBodyResponse(t, "application/rss+xml", "testdata/rss.xml")
	reqBody := url.Values{
		"url": {ts.URL},
	}
	resp, err := http.PostForm(tsHandler.URL, reqBody)
	if err != nil {
		t.Fatal(err)
	}
	got := resp.StatusCode
	if want != got {
		t.Fatalf("want status code %d, got %d", want, got)
	}
}

func TestHandleFeeds_AnswersBadRequestGivenRequestWithMethodPostAndNoBody(t *testing.T) {
	t.Parallel()
	want := http.StatusBadRequest
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	ts := httptest.NewServer(http.HandlerFunc(m.HandleFeeds))
	defer ts.Close()
	resp, err := http.PostForm(ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := resp.StatusCode
	if want != got {
		t.Fatalf("want status code %d, got %d", want, got)
	}
}

func TestHandleFeeds_AnswersBadRequestGivenRequestWithMethodPostAndBlankSpacesInBodyURL(t *testing.T) {
	t.Parallel()
	want := http.StatusBadRequest
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	ts := httptest.NewServer(http.HandlerFunc(m.HandleFeeds))
	defer ts.Close()
	reqBody := url.Values{
		"url": {" "},
	}
	resp, err := http.PostForm(ts.URL, reqBody)
	if err != nil {
		t.Fatal(err)
	}
	got := resp.StatusCode
	if want != got {
		t.Fatalf("want status code %d, got %d", want, got)
	}
}

func TestHandleFeeds_DeletesFeedGivenDeleteReqiuestAndPrePopulatedStore(t *testing.T) {
	t.Parallel()
	want := []morningpost.Feed{
		{Endpoint: "https://fake-https.url"},
	}

	f := fakeStore{
		0: {
			Endpoint: "http://fake-http.url",
		},
		1: {
			Endpoint: "https://fake-https.url",
		},
	}
	m, err := morningpost.New(
		f,
		morningpost.WithStderr(io.Discard),
		morningpost.WithStdout(io.Discard),
	)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(http.HandlerFunc(m.HandleFeeds))
	defer ts.Close()
	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/0", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected response status code %q", resp.Status)
	}
	got := m.Store.GetAll()
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestHandleFeeds_AddsFeedGivenPostRequest(t *testing.T) {
	t.Parallel()
	epTs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("content-type", "application/rss+xml")
		fmt.Fprint(w, "<rss></rss>")
	}))
	want := []morningpost.Feed{
		{
			Endpoint: epTs.URL,
			Type:     morningpost.FeedTypeRSS,
		},
	}
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	ts := httptest.NewServer(http.HandlerFunc(m.HandleFeeds))
	defer ts.Close()
	resp, err := http.PostForm(ts.URL, url.Values{"url": []string{epTs.URL}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected response status code %q", resp.Status)
	}
	got := m.Store.GetAll()
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestHandleHome_AnswersMethodNotAllowedGivenRequestWithBogusMethod(t *testing.T) {
	t.Parallel()
	want := http.StatusMethodNotAllowed
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	ts := httptest.NewServer(http.HandlerFunc(m.HandleHome))
	defer ts.Close()
	req, err := http.NewRequest("bogus", ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	got := resp.StatusCode
	if want != got {
		t.Fatalf("want status code %d, got %d", want, got)
	}
}

func TestFeedGetNews_ReturnsNewsFromFeed(t *testing.T) {
	testCases := []struct {
		contentType string
		desc        string
	}{
		{
			contentType: "application/rss+xml",
			desc:        "Given Response Content-Type application/rss+xml and RSS body",
		},
		{
			contentType: "application/xml",
			desc:        "Given Response Content-Type application/xml and RSS body",
		},
		{
			contentType: "text/xml; charset=UTF-8 and RSS body",
			desc:        "Given Response Content-Type text/xml",
		},
	}
	want := []morningpost.News{
		{
			Feed:  "FeedForAll Sample Feed",
			Title: "RSS Solutions for Restaurants",
			URL:   "http://www.feedforall.com/restaurant.htm",
		},
		{
			Feed:  "FeedForAll Sample Feed",
			Title: "RSS Solutions for Schools and Colleges",
			URL:   "http://www.feedforall.com/schools.htm",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			ts := newServerWithContentTypeAndBodyResponse(t, tC.contentType, "testdata/rss.xml")
			feed := morningpost.Feed{
				Endpoint: ts.URL,
				Type:     morningpost.FeedTypeRSS,
			}
			got, err := feed.GetNews()
			if err != nil {
				t.Fatal(err)
			}
			if !cmp.Equal(want, got) {
				t.Fatal(cmp.Diff(want, got))
			}
		})
	}
}

func TestFeedGetNews_ReturnsNewsFromFeedGivenResponseContentTypeApplicationAtomXMLAndAtomBody(t *testing.T) {
	want := []morningpost.News{
		{
			Feed:  "Chris's Wiki :: blog",
			Title: "ZFS on Linux and when you get stale NFSv3 mounts",
			URL:   "https://utcc.utoronto.ca/~cks/space/blog/linux/ZFSAndNFSMountInvalidation",
		},
		{
			Feed:  "Chris's Wiki :: blog",
			Title: "Debconf's questions, or really whiptail, doesn't always work in xterms",
			URL:   "https://utcc.utoronto.ca/~cks/space/blog/linux/DebconfWhiptailVsXterm",
		},
	}
	ts := newServerWithContentTypeAndBodyResponse(t, "application/atom+xml", "testdata/atom.xml")
	feed := morningpost.Feed{
		Endpoint: ts.URL,
		Type:     morningpost.FeedTypeAtom,
	}
	got, err := feed.GetNews()
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestFeedGetNews_ReturnsNewsFromFeedGivenResponseContentTypeApplicationTextXMLAndRDFBody(t *testing.T) {
	want := []morningpost.News{
		{
			Feed:  "Slashdot",
			Title: "Ask Slashdot:  What Was Your Longest-Lived PC?",
			URL:   "https://ask.slashdot.org/story/23/04/02/0058226/ask-slashdot-what-was-your-longest-lived-pc?utm_source=rss1.0mainlinkanon&utm_medium=feed",
		},
		{
			Feed:  "Slashdot",
			Title: "San Francisco Faces 'Doom Loop' from Office Workers Staying Home, Gutting Tax Base",
			URL:   "https://it.slashdot.org/story/23/04/01/2059224/san-francisco-faces-doom-loop-from-office-workers-staying-home-gutting-tax-base?utm_source=rss1.0mainlinkanon&utm_medium=feed",
		},
	}
	ts := newServerWithContentTypeAndBodyResponse(t, "text/xml", "testdata/rdf.xml")
	feed := morningpost.Feed{
		Endpoint: ts.URL,
		Type:     morningpost.FeedTypeRDF,
	}
	got, err := feed.GetNews()
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestFeedGetNews_SetsHeadersOnHTTPRequest(t *testing.T) {
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
	feed := morningpost.Feed{
		Endpoint: ts.URL,
		Type:     morningpost.FeedTypeRSS,
	}
	_, err := feed.GetNews()
	if err != nil {
		t.Fatal(err)
	}
}

func TestFeedGetNews_ErrorsGivenInvalidEndpoint(t *testing.T) {
	t.Parallel()
	feed := morningpost.Feed{
		Endpoint: "bogus://",
	}
	_, err := feed.GetNews()
	if err == nil {
		t.Fatal("want error but got nil")
	}
}

func TestFeedGetNews_ErrorsIfResponseContentTypeIsUnexpected(t *testing.T) {
	t.Parallel()
	ts := newServerWithContentTypeResponse(t, "bogus")
	feed := morningpost.Feed{
		Endpoint: ts.URL,
	}
	_, err := feed.GetNews()
	if err == nil {
		t.Fatal("want error but got nil")
	}
}

func TestGetNews_ReturnsNewsFromAllFeedsGivenPopulatedStore(t *testing.T) {
	t.Parallel()
	want := generateNews(2)
	ts1 := newServerWithContentTypeAndBodyResponse(t, "application/rss+xml", "testdata/rss-with-one-news-1.xml")
	ts2 := newServerWithContentTypeAndBodyResponse(t, "application/rss+xml", "testdata/rss-with-one-news-2.xml")
	m, err := morningpost.New(fakeStore{
		0: morningpost.Feed{
			Endpoint: ts1.URL,
			Type:     morningpost.FeedTypeRSS,
		},
		1: morningpost.Feed{
			Endpoint: ts2.URL,
			Type:     morningpost.FeedTypeRSS,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = m.GetNews()
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(m.News, func(i, j int) bool {
		return m.News[i].Title < m.News[j].Title
	})
	got := m.News
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestGetNews_CleansUpNewsOnEachExecution(t *testing.T) {
	t.Parallel()
	want := 0
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	m.News = []morningpost.News{{
		Title: "I hope it disappear",
		URL:   "http://fake.url/fake-news",
	}}
	err := m.GetNews()
	if err != nil {
		t.Fatal(err)
	}
	got := len(m.News)
	if want != got {
		t.Fatalf("want number of news %d, got %d", want, got)
	}
}

func TestGetNews_ErrorsIfFeedGetNewsErrors(t *testing.T) {
	t.Parallel()
	m, err := morningpost.New(fakeStore{
		0: morningpost.Feed{
			Endpoint: "bogus://",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = m.GetNews()
	if err == nil {
		t.Fatal("want error but got nil")
	}
}

func TestParseAtomResponse_ReturnsNewsGivenAtomWithTwoNews(t *testing.T) {
	t.Parallel()
	want := []morningpost.News{
		{
			Feed:  "Chris's Wiki :: blog",
			Title: "ZFS on Linux and when you get stale NFSv3 mounts",
			URL:   "https://utcc.utoronto.ca/~cks/space/blog/linux/ZFSAndNFSMountInvalidation",
		},
		{
			Feed:  "Chris's Wiki :: blog",
			Title: "Debconf's questions, or really whiptail, doesn't always work in xterms",
			URL:   "https://utcc.utoronto.ca/~cks/space/blog/linux/DebconfWhiptailVsXterm",
		},
	}
	file, err := os.Open("testdata/atom.xml")
	if err != nil {
		t.Fatalf("Cannot open file testdata/atom.xml: %+v", err)
	}
	got, err := morningpost.ParseAtomResponse(file)
	if err != nil {
		t.Fatalf("Cannot parse content: %+v", err)
	}
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestParseAtomResponse_ErrorsIfDataIsNotXML(t *testing.T) {
	t.Parallel()
	_, err := morningpost.ParseAtomResponse(strings.NewReader("{}"))
	if err == nil {
		t.Fatalf("want error but not found")
	}
}

func TestHandleHome_AnswersNotFoundForUnkownRoute(t *testing.T) {
	t.Parallel()
	want := http.StatusNotFound
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	ts := httptest.NewServer(http.HandlerFunc(m.HandleHome))
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/bogus")
	if err != nil {
		t.Fatal(err)
	}
	got := resp.StatusCode
	if want != got {
		t.Fatalf("want response status code %d, got %d", want, got)
	}
}

func TestAddNews_AddsAllNewsGivenConcurrentAccess(t *testing.T) {
	t.Parallel()
	want := 1000
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	allNews := generateNews(1000)
	var wg sync.WaitGroup
	for x := 0; x < 1000; x++ {
		wg.Add(1)
		go func(x int) {
			m.AddNews([]morningpost.News{allNews[x]})
			wg.Done()
		}(x)
	}
	wg.Wait()
	got := len(m.News)
	if want != got {
		t.Fatalf("want %d news, got %d", want, got)
	}
}

func TestParseLinkTags_ReturnsFeedEndpointGivenHTMLPageWithRSSFeedInBodyElement(t *testing.T) {
	t.Parallel()
	want := []morningpost.Feed{{
		Endpoint: "https://bitfieldconsulting.com/golang?format=rss",
		Type:     morningpost.FeedTypeRSS,
	}}
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
	want := []morningpost.Feed{{
		Endpoint: "http://fake.url/rss",
		Type:     morningpost.FeedTypeRSS,
	}}
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
	want := []morningpost.Feed{{
		Endpoint: "http://fake.url/feed/",
		Type:     morningpost.FeedTypeAtom,
	}}
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

func TestFindFeeds_ErrorsIfStatusCodeIsNotStatusOK(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer ts.Close()
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	_, err := m.FindFeeds(ts.URL)
	if err == nil {
		t.Fatal("want error but got nil")
	}
}

func TestFindFeeds_ErrorsGivenURLWithSchemeBogus(t *testing.T) {
	t.Parallel()
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	_, err := m.FindFeeds("bogus://")
	if err == nil {
		t.Fatal("want error but got nil")
	}
}

func TestFindFeeds_ErrorsGivenUnexpectedContentType(t *testing.T) {
	t.Parallel()
	ts := newServerWithContentTypeResponse(t, "bogus")
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	_, err := m.FindFeeds(ts.URL)
	if err == nil {
		t.Fatal("want error but got nil")
	}
}

func TestFindFeeds_ReturnsExpectedFeedsGivenApplicationRSSXMLContentType(t *testing.T) {
	t.Parallel()
	ts := newServerWithContentTypeAndBodyResponse(t, "application/rss+xml", "testdata/rss.xml")
	want := []morningpost.Feed{{
		Endpoint: ts.URL,
		Type:     morningpost.FeedTypeRSS,
	}}
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	got, err := m.FindFeeds(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestFindFeeds_ReturnsExpectedFeedsGivenApplicationXMLContentTypeAndRSSData(t *testing.T) {
	t.Parallel()
	ts := newServerWithContentTypeAndBodyResponse(t, "application/xml", "testdata/rss.xml")
	want := []morningpost.Feed{{
		Endpoint: ts.URL,
		Type:     morningpost.FeedTypeRSS,
	}}
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	got, err := m.FindFeeds(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestFindFeeds_ReturnsExpectedFeedsGivenTextXMLContentTypeAndRSSData(t *testing.T) {
	t.Parallel()
	ts := newServerWithContentTypeAndBodyResponse(t, "text/xml", "testdata/rss.xml")
	want := []morningpost.Feed{{
		Endpoint: ts.URL,
		Type:     morningpost.FeedTypeRSS,
	}}
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	got, err := m.FindFeeds(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestFindFeeds_ReturnsExpectedFeedsGivenTextXMLContentTypeAndRDFData(t *testing.T) {
	t.Parallel()
	ts := newServerWithContentTypeAndBodyResponse(t, "text/xml", "testdata/rdf.xml")
	want := []morningpost.Feed{{
		Endpoint: ts.URL,
		Type:     morningpost.FeedTypeRDF,
	}}
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	got, err := m.FindFeeds(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestFindFeeds_ReturnsExpectedFeedsGivenAtomApplicationContentType(t *testing.T) {
	t.Parallel()
	ts := newServerWithContentTypeAndBodyResponse(t, "application/atom+xml", "testdata/atom.xml")
	want := []morningpost.Feed{{
		Endpoint: ts.URL,
		Type:     "Atom",
	}}
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	got, err := m.FindFeeds(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestFindFeeds_ReturnsExpectedFeedsGivenHTMLPageWithFeedsInFullLinkFormat(t *testing.T) {
	t.Parallel()
	want := []morningpost.Feed{
		{
			Endpoint: "http://example.com/rss",
			Type:     morningpost.FeedTypeRSS,
		},
		{
			Endpoint: "http://example.com/atom",
			Type:     morningpost.FeedTypeAtom,
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
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	got, err := m.FindFeeds(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
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
			Endpoint: ts.URL + "/rss",
			Type:     morningpost.FeedTypeRSS,
		},
	}
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	got, err := m.FindFeeds(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
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
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	_, err := m.FindFeeds(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
}

func TestNewNews_ErrorsGiven(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		desc  string
		feed  string
		title string
		URL   string
	}{
		{
			desc:  "Empty Feed",
			feed:  "",
			title: "bogus",
			URL:   "http://fake.url",
		},
		{
			desc:  "Empty Title",
			feed:  "bogus",
			title: "",
			URL:   "http://fake.url",
		},
		{
			desc:  "Empty URL",
			feed:  "bogus",
			title: "bogus",
			URL:   "",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			_, err := morningpost.NewNews(tC.feed, tC.title, tC.URL)
			if err == nil {
				t.Fatal("want error but got nil")
			}
		})
	}
}

func TestHandleNews_RenderProperHTMLPageGivenGetRequestOnPageOne(t *testing.T) {
	t.Parallel()
	want := []byte(`<tr>
  <th class="table-light" scope="row">
    <a href="http://fake.url/title1.htm">Title 1</a>
    <small class="text-muted">Unit Test Feed</small>
  </th>
</tr>
<tr
  hx-get="/news?page=2"
  hx-trigger="revealed"
  hx-swap="afterend"
>
  <th class="table-light" scope="row">
    <a href="http://fake.url/title2.htm">Title 2</a>
    <small class="text-muted">Unit Test Feed</small>
  </th>
</tr>
`)
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	m.NewsPageSize = 2
	m.PageNews = []morningpost.News{
		{
			Feed:  "Unit Test Feed",
			Title: "Title 1",
			URL:   "http://fake.url/title1.htm",
		},
		{
			Feed:  "Unit Test Feed",
			Title: "Title 2",
			URL:   "http://fake.url/title2.htm",
		},
		{
			Feed:  "Unit Test Feed",
			Title: "Title 3",
			URL:   "http://fake.url/title3.htm",
		},
		{
			Feed:  "Unit Test Feed",
			Title: "Title 4",
			URL:   "http://fake.url/title4.htm",
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(m.HandleNews))
	defer ts.Close()
	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	got := removeEmptyLines(body)
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestHandleNews_RenderProperHTMLPageGivenRequestLastPage(t *testing.T) {
	t.Parallel()
	want := []byte(`<tr>
  <th class="table-light" scope="row">
    <a href="http://fake.url/title3.htm">Title 3</a>
    <small class="text-muted">Unit Test Feed</small>
  </th>
</tr>
<tr>
  <th class="table-light" scope="row">
    <a href="http://fake.url/title4.htm">Title 4</a>
    <small class="text-muted">Unit Test Feed</small>
  </th>
</tr>
`)
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	m.NewsPageSize = 2
	m.PageNews = []morningpost.News{
		{
			Feed:  "Unit Test Feed",
			Title: "Title 1",
			URL:   "http://fake.url/title1.htm",
		},
		{
			Feed:  "Unit Test Feed",
			Title: "Title 2",
			URL:   "http://fake.url/title2.htm",
		},
		{
			Feed:  "Unit Test Feed",
			Title: "Title 3",
			URL:   "http://fake.url/title3.htm",
		},
		{
			Feed:  "Unit Test Feed",
			Title: "Title 4",
			URL:   "http://fake.url/title4.htm",
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(m.HandleNews))
	defer ts.Close()
	resp, err := http.Get(ts.URL + "?page=2")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	got := removeEmptyLines(body)
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestHandleNews_AnswersMethodNotAllowedGivenRequestWithUnexpectedMethod(t *testing.T) {
	t.Parallel()
	want := http.StatusMethodNotAllowed
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	ts := httptest.NewServer(http.HandlerFunc(m.HandleNews))
	defer ts.Close()
	req, err := http.NewRequest("bogus", ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	got := resp.StatusCode
	if want != got {
		t.Fatalf("want status code %d, got %d", want, got)
	}
}

func TestServe_RespondsStatusOKOnIndexGivenEmptyStore(t *testing.T) {
	t.Parallel()
	want := http.StatusOK
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	l, err := nettest.NewLocalListener("tcp")
	if err != nil {
		t.Fatal(err)
	}
	go m.Serve(l)
	waitServerHealthCheck(t, l.Addr().String())
	resp, err := http.Get("http://" + l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	got := resp.StatusCode
	if want != got {
		t.Fatalf("unexpected response status code %q", resp.Status)
	}
}

func TestServe_ReturnsExpectedBodyOnIndexGivenEmptyStore(t *testing.T) {
	t.Parallel()
	want, err := os.ReadFile("testdata/golden/index.html")
	if err != nil {
		t.Fatal(err)
	}
	l, err := nettest.NewLocalListener("tcp")
	if err != nil {
		t.Fatal(err)
	}
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	go m.Serve(l)
	waitServerHealthCheck(t, l.Addr().String())
	resp, err := http.Get("http://" + l.Addr().String() + "/")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected response status code %q", resp.Status)
	}
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestServe_RespondsStatusOKOnFeedsGivenEmptyStore(t *testing.T) {
	t.Parallel()
	want := http.StatusOK
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	l, err := nettest.NewLocalListener("tcp")
	if err != nil {
		t.Fatal(err)
	}
	go m.Serve(l)
	waitServerHealthCheck(t, l.Addr().String())
	resp, err := http.Get("http://" + l.Addr().String() + "/feeds/")
	if err != nil {
		t.Fatal(err)
	}
	got := resp.StatusCode
	if want != got {
		t.Fatalf("unexpected response status code %q", resp.Status)
	}
}

func TestServe_ReturnsExpectedBodyOnFeedsGivenEmptyStore(t *testing.T) {
	t.Parallel()
	want, err := os.ReadFile("testdata/golden/feeds.html")
	if err != nil {
		t.Fatal(err)
	}
	l, err := nettest.NewLocalListener("tcp")
	if err != nil {
		t.Fatal(err)
	}
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	go m.Serve(l)
	waitServerHealthCheck(t, l.Addr().String())
	resp, err := http.Get("http://" + l.Addr().String() + "/feeds/")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected response status code %q", resp.Status)
	}
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestServe_RespondsStatusOKOnNewsGivenEmptyStore(t *testing.T) {
	t.Parallel()
	want := http.StatusOK
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	l, err := nettest.NewLocalListener("tcp")
	if err != nil {
		t.Fatal(err)
	}
	go m.Serve(l)
	waitServerHealthCheck(t, l.Addr().String())
	resp, err := http.Get("http://" + l.Addr().String() + "/news/")
	if err != nil {
		t.Fatal(err)
	}
	got := resp.StatusCode
	if want != got {
		t.Fatalf("unexpected response status code %q", resp.Status)
	}
}

func TestServe_ReturnsExpectedBodyOnNewsGivenEmptyStore(t *testing.T) {
	t.Parallel()
	want := []byte("\n\n\n")
	l, err := nettest.NewLocalListener("tcp")
	if err != nil {
		t.Fatal(err)
	}
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	go m.Serve(l)
	waitServerHealthCheck(t, l.Addr().String())
	resp, err := http.Get("http://" + l.Addr().String() + "/news/")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected response status code %q", resp.Status)
	}
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

// func TestRunServer_ServeHTTPRequests(t *testing.T) {
// 	t.Parallel()
// 	want := http.StatusOK
// 	go morningpost.RunServer(io.Discard, io.Discard, []string{"-l", ":38000"}...)
// 	for x := 0; x < 10; x++ {
// 		resp, err := http.Get("http://127.0.0.1:38000")
// 		if err != nil {
// 			time.Sleep(300 * time.Millisecond)
// 			continue
// 		}
// 		if resp.StatusCode != want {
// 			time.Sleep(300 * time.Millisecond)
// 			continue
// 		}
// 		return
// 	}
// 	t.Fatal("server at http://127.0.0.1:38000 is not responding to GET /")
// }
