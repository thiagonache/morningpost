package morningpost

import (
	"encoding/xml"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/charset"
)

const (
	DefaultHTTPTimeout = 30 * time.Second
	FeedTypeAtom       = "Atom"
	FeedTypeRDF        = "RDF"
	FeedTypeRSS        = "RSS"
)

type Feed struct {
	Type string
	URL  string
}

type MorningPost struct {
	Client   *http.Client
	Handlers map[string]func(http.ResponseWriter, *http.Request)
	store    Store
}

func (m *MorningPost) FindFeeds(URL string) []Feed {
	req, err := http.NewRequest(http.MethodGet, URL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("user-agent", "MorningPost/0.1")
	req.Header.Set("accept", "*/*")
	resp, err := m.Client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	contentType := parseContentType(resp.Header)
	switch contentType {
	case "application/rss+xml", "application/atom+xml", "text/xml", "application/xml":
		feedType, err := ParseFeedType(resp.Body)
		if err != nil {
			return nil
		}
		return []Feed{{
			URL:  URL,
			Type: feedType,
		}}
	case "text/html":
		feeds, err := ParseLinkTags(resp.Body, URL)
		if err != nil {
			return nil
		}
		return feeds
	default:
		return nil
	}
}

func (m *MorningPost) HandleRouteVisit(w http.ResponseWriter, r *http.Request) {
	uriParts := strings.Split(r.RequestURI, "/")
	URL, err := url.PathUnescape(uriParts[len(uriParts)-1])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodHead:
	case http.MethodGet:
		if !m.store.URLVisited(URL) {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodPost:
		m.store.RecordVisit(URL)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func (m *MorningPost) HandleRouteFeedCRUD(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodHead:
	case http.MethodGet:
		uriParts := strings.Split(r.RequestURI, "/")
		URL, err := url.PathUnescape(uriParts[len(uriParts)-1])
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !m.store.FeedExists(URL) {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		uriParts := strings.Split(r.RequestURI, "/")
		URL, err := url.PathUnescape(uriParts[len(uriParts)-1])
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		m.store.DeleteFeed(URL)
		w.WriteHeader(http.StatusNoContent)
	case http.MethodPost:
		uriParts := strings.Split(r.RequestURI, "/")
		URL, err := url.PathUnescape(uriParts[len(uriParts)-1])
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		feeds := m.FindFeeds(URL)
		for _, feed := range feeds {
			m.store.AddFeed(feed)
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func (m *MorningPost) HandleRouteFeed(w http.ResponseWriter, r *http.Request) {
	switch r.RequestURI {
	case "/feed/table/rows":
		m.HandleRouteFeedTableRows(w, r)
	default:
		m.HandleRouteFeedCRUD(w, r)
	}
}

func (m *MorningPost) HandleRouteFeedTableRows(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tpl := template.Must(template.New("feeds-table-rows.gohtml").Funcs(template.FuncMap{
			"PathEscape": url.PathEscape,
		}).ParseFiles("templates/feeds-table-rows.gohtml"))
		feeds := m.store.GetFeeds()
		sort.Slice(feeds, func(i, j int) bool {
			return feeds[i].URL < feeds[j].URL
		})
		err := tpl.Execute(w, feeds)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func (m *MorningPost) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for pattern, handler := range m.Handlers {
		if strings.HasPrefix(r.RequestURI, pattern) {
			handler(w, r)
			return
		}
	}
	http.NotFound(w, r)
}

func New(store Store) *MorningPost {
	m := &MorningPost{
		Client: &http.Client{
			Timeout: DefaultHTTPTimeout,
		},
		store: store,
	}
	m.Handlers = map[string]func(http.ResponseWriter, *http.Request){
		"/visit/": m.HandleRouteVisit,
		"/feed/":  m.HandleRouteFeed,
	}
	return m
}

func parseContentType(headers http.Header) string {
	return strings.Split(headers.Get("content-type"), ";")[0]
}

func ParseFeedType(r io.Reader) (string, error) {
	type feedType struct {
		XMLName xml.Name
	}
	feedTypeData := feedType{}
	decoder := xml.NewDecoder(r)
	decoder.CharsetReader = charset.NewReaderLabel
	err := decoder.Decode(&feedTypeData)
	if err != nil {
		return "", err
	}
	switch strings.ToUpper(feedTypeData.XMLName.Local) {
	case "RSS":
		return FeedTypeRSS, nil
	case "FEED":
		return FeedTypeAtom, nil
	case "RDF":
		return FeedTypeRDF, nil
	default:
		return "", fmt.Errorf("unexpected XMLName %q", strings.ToUpper(feedTypeData.XMLName.Local))
	}
}

func ParseLinkTags(r io.Reader, baseURL string) ([]Feed, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}
	feeds := []Feed{}
	doc.Find("link[type='application/rss+xml']").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			u, err := url.Parse(href)
			if err != nil {
				return
			}
			feeds = append(feeds, Feed{
				URL:  base.ResolveReference(u).String(),
				Type: FeedTypeRSS,
			})
		}
	})
	doc.Find("link[type='application/atom+xml']").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			u, err := url.Parse(href)
			if err != nil {
				return
			}
			feeds = append(feeds, Feed{
				URL:  base.ResolveReference(u).String(),
				Type: FeedTypeAtom,
			})
		}
	})
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		title, _ := s.Attr("title")
		if strings.Contains(strings.ToLower(title), "rss") {
			href, exists := s.Attr("href")
			if exists {
				u, err := url.Parse(href)
				if err != nil {
					return
				}
				feeds = append(feeds, Feed{
					URL:  base.ResolveReference(u).String(),
					Type: FeedTypeRSS,
				})
			}
		}
	})
	return feeds, nil
}
