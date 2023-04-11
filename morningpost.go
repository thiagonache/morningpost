package morningpost

import (
	"context"
	"embed"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/charset"
	"golang.org/x/sync/errgroup"
)

//go:embed templates/*.gohtml
var templates embed.FS

const (
	DefaultHTTPTimeout  = 30 * time.Second
	DefaultListenPort   = 33000
	DefaultNewsPageSize = 10
	FeedTypeAtom        = "Atom"
	FeedTypeRDF         = "RDF"
	FeedTypeRSS         = "RSS"
)

type News struct {
	Feed  string
	Title string
	URL   string
}

func NewNews(feed, title, URL string) (News, error) {
	if feed == "" || title == "" || URL == "" {
		return News{}, errors.New("empty feed, title or url")
	}
	return News{
		Feed:  feed,
		Title: title,
		URL:   URL,
	}, nil
}

type MorningPost struct {
	Client       *http.Client
	ctx          context.Context
	fileStore    *FileStore
	ListenPort   int
	NewsPageSize int
	PageNews     []News
	Server       *http.Server
	Stderr       io.Writer
	Stdout       io.Writer
	stop         context.CancelFunc

	mu   *sync.Mutex
	News []News
}

func (m *MorningPost) GetNews() error {
	m.EmptyNews()
	defer m.RandomNews()
	g := new(errgroup.Group)
	for _, feed := range m.fileStore.GetAll() {
		feed := feed
		g.Go(func() error {
			news, err := feed.GetNews()
			if err != nil {
				return fmt.Errorf("%q: %w", feed.Endpoint, err)
			}
			m.AddNews(news)
			return nil
		})

	}
	return g.Wait()
}

func (m *MorningPost) RandomNews() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PageNews = make([]News, 0, m.NewsPageSize)
	randomIndexes := rand.Perm(len(m.News))
	for _, idx := range randomIndexes {
		m.PageNews = append(m.PageNews, m.News[idx])
	}
}

func (m *MorningPost) AddNews(news []News) {
	m.mu.Lock()
	m.News = append(m.News, news...)
	m.mu.Unlock()
}

func (m *MorningPost) EmptyNews() {
	m.mu.Lock()
	m.News = []News{}
	m.mu.Unlock()
}

func (m *MorningPost) ReadURLFromForm(r *http.Request) (string, error) {
	err := r.ParseForm()
	if err != nil {
		return "", err
	}
	url := r.Form.Get("url")
	url = strings.TrimSpace(url)
	if url == "" {
		return "", errors.New("bad Request: please, inform the URL")
	}
	return url, nil
}

func (m *MorningPost) ReadFeedIDFromURI(uri string) string {
	urlParts := strings.Split(uri, "/")
	return urlParts[len(urlParts)-1]
}

func (m *MorningPost) FindFeeds(URL string) ([]Feed, error) {
	req, err := http.NewRequest(http.MethodGet, URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("user-agent", "MorningPost/0.1")
	req.Header.Set("accept", "*/*")
	resp, err := m.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response status code %q", resp.Status)
	}
	contentType := parseContentType(resp.Header)
	switch contentType {
	case "application/rss+xml", "application/atom+xml", "text/xml", "application/xml":
		feedType, err := ParseFeedType(resp.Body)
		if err != nil {
			return nil, err
		}
		return []Feed{{
			Endpoint: URL,
			Type:     feedType,
		}}, nil
	case "text/html":
		feeds, err := ParseLinkTags(resp.Body, URL)
		if err != nil {
			return nil, err
		}
		return feeds, nil
	default:
		return nil, fmt.Errorf("unexpected content type: %q", contentType)
	}
}

func (m *MorningPost) HandleFeeds(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(m.Stdout, r.Method, r.URL)
	switch r.Method {
	case http.MethodHead:
		w.WriteHeader(http.StatusOK)
	case http.MethodPost:
		URL, err := m.ReadURLFromForm(r)
		if err != nil {
			fmt.Fprintln(m.Stderr, err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		feeds, err := m.FindFeeds(URL)
		if err != nil {
			fmt.Fprintln(m.Stderr, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, feed := range feeds {
			m.fileStore.Add(feed)
		}
		err = RenderHTMLTemplate(w, "templates/feeds.gohtml", m.fileStore.GetAll())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case http.MethodGet:
		err := RenderHTMLTemplate(w, "templates/feeds.gohtml", m.fileStore.GetAll())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case http.MethodDelete:
		id := m.ReadFeedIDFromURI(r.URL.Path)
		ui64, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		m.fileStore.Delete(ui64)
	default:
		fmt.Fprintln(m.Stderr, "Method not allowed")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *MorningPost) HandleHome(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(m.Stdout, r.Method, r.URL)
	if r.RequestURI != "/" {
		fmt.Fprintf(m.Stderr, "%s not found\n", r.RequestURI)
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		err := m.GetNews()
		if err != nil {
			fmt.Fprintln(m.Stderr, err.Error())
		}
		err = RenderHTMLTemplate(w, "templates/home.gohtml", m.PageNews)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		fmt.Fprintln(m.Stderr, "Method not allowed")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *MorningPost) HandleNews(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(m.Stdout, r.Method, r.URL)
	switch r.Method {
	case http.MethodGet:
		page := 1
		params := r.URL.Query()
		if params.Get("page") != "" {
			var err error
			page, err = strconv.Atoi(params.Get("page"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		nextPage := page + 1
		lastIdx := m.NewsPageSize * page
		if lastIdx > len(m.PageNews) {
			lastIdx = len(m.PageNews)
		}
		if lastIdx >= len(m.PageNews) {
			nextPage = 0
		}
		data := struct {
			LastPageIdx int
			NextPage    int
			PageNews    []News
		}{
			m.NewsPageSize - 1,
			nextPage,
			m.PageNews[m.NewsPageSize*(page-1) : lastIdx],
		}
		tpl := template.Must(template.ParseFS(templates, "templates/news.gohtml"))
		err := tpl.Execute(w, data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		fmt.Fprintln(m.Stderr, "Method not allowed")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

func (m *MorningPost) Run() error {
	err := m.fileStore.Load()
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", m.HandleHome)
	mux.HandleFunc("/feeds/", m.HandleFeeds)
	mux.HandleFunc("/news/", m.HandleNews)
	m.Server = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", m.ListenPort),
		Handler: mux,
	}
	fmt.Fprintf(m.Stdout, "Listening at http://%s\n", fmt.Sprintf("127.0.0.1:%d", m.ListenPort))
	return m.Server.ListenAndServe()
}

func (m *MorningPost) Shutdown() error {
	err := m.Server.Shutdown(m.ctx)
	if err != nil && err.Error() != context.Canceled.Error() {
		fmt.Fprintf(m.Stderr, "Error running server shutdown: %+v", err)
	}
	return m.fileStore.Save()
}

func (m *MorningPost) WaitForExit() error {
	<-m.ctx.Done()
	fmt.Fprintln(m.Stdout, "Please WAIT! Do not repeat this action")
	return m.Shutdown()
}

func (m *MorningPost) FileStorePath() string {
	return m.fileStore.Path
}

func (m *MorningPost) FileStoreData() map[uint64]Feed {
	return m.fileStore.Data
}

func New(opts ...Option) (*MorningPost, error) {
	rand.Seed(time.Now().UTC().UnixNano())
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	m := &MorningPost{
		Client: &http.Client{
			Timeout: DefaultHTTPTimeout,
		},
		ctx: ctx,
		fileStore: &FileStore{
			Data: map[uint64]Feed{},
			Path: userStateDir() + "/MorningPost/morningpost.db",
		},
		ListenPort:   DefaultListenPort,
		mu:           &sync.Mutex{},
		NewsPageSize: DefaultNewsPageSize,
		Stderr:       os.Stderr,
		Stdout:       os.Stdout,
		stop:         stop,
	}
	for _, o := range opts {
		err := o(m)
		if err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(path.Dir(m.fileStore.Path)); os.IsNotExist(err) {
		err := os.MkdirAll(path.Dir(m.fileStore.Path), 0755)
		if err != nil {
			return nil, err
		}
	}
	return m, nil
}

func getenv(key string) string {
	v, _ := syscall.Getenv(key)
	return v
}

func userStateDir() string {
	switch runtime.GOOS {
	case "windows":
		dir := getenv("AppData")
		if dir == "" {
			return "./"
		}
		return dir
	case "darwin", "ios":
		dir := getenv("HOME")
		if dir == "" {
			return "./"
		}
		dir += "/Library/Application Support"
		return dir
	default: // Unix
		dir := getenv("XDG_STATE_HOME")
		if dir == "" {
			return "/var/lib"
		}
		return dir
	}
}

func Main() int {
	m, err := New(
		FromArgs(os.Args[1:]),
	)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	go m.Run()
	err = m.WaitForExit()
	if err != nil {
		fmt.Fprintln(m.Stderr, err)
		return 1
	}
	fmt.Fprintln(m.Stdout, "Done. Thank you! <3")
	return 0
}

type Feed struct {
	Endpoint string
	ID       uint64
	Type     string
}

func (f Feed) GetNews() ([]News, error) {
	req, err := http.NewRequest(http.MethodGet, f.Endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("user-agent", "MorningPost/0.1")
	req.Header.Set("accept", "*/*")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	switch f.Type {
	case FeedTypeRSS:
		return ParseRSSResponse(resp.Body)
	case FeedTypeRDF:
		return ParseRDFResponse(resp.Body)
	case FeedTypeAtom:
		return ParseAtomResponse(resp.Body)
	default:
		return nil, fmt.Errorf("unkown feed type %q", f.Type)
	}
}

func parseContentType(headers http.Header) string {
	return strings.Split(headers.Get("content-type"), ";")[0]
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
				Endpoint: base.ResolveReference(u).String(),
				Type:     FeedTypeRSS,
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
				Endpoint: base.ResolveReference(u).String(),
				Type:     FeedTypeAtom,
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
					Endpoint: base.ResolveReference(u).String(),
					Type:     FeedTypeRSS,
				})
			}
		}
	})
	return feeds, nil
}

func RenderHTMLTemplate(w io.Writer, templatePath string, data any) error {
	tpl := template.Must(template.New("main").ParseFS(templates, "templates/base.gohtml", templatePath))
	err := tpl.Execute(w, data)
	if err != nil {
		return err
	}
	return nil
}

func ParseRSSResponse(r io.Reader) ([]News, error) {
	type rss struct {
		XMLName xml.Name `xml:"rss"`
		Channel struct {
			Title string `xml:"title"`
			Items []struct {
				Title string `xml:"title"`
				Link  string `xml:"link"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	rssData := rss{}
	decoder := xml.NewDecoder(r)
	decoder.CharsetReader = charset.NewReaderLabel
	err := decoder.Decode(&rssData)
	if err != nil {
		return nil, fmt.Errorf("cannot decode data: %w", err)
	}
	allNews := make([]News, 0, len(rssData.Channel.Items))
	for _, item := range rssData.Channel.Items {
		news, err := NewNews(rssData.Channel.Title, item.Title, item.Link)
		if err != nil {
			continue
		}
		allNews = append(allNews, news)
	}
	return allNews, nil
}

func ParseRDFResponse(r io.Reader) ([]News, error) {
	type rdf struct {
		XMLName xml.Name `xml:"RDF"`
		Channel struct {
			Title string `xml:"title"`
		} `xml:"channel"`
		Items []struct {
			Title string `xml:"title"`
			Link  string `xml:"link"`
		} `xml:"item"`
	}
	rdfData := rdf{}
	decoder := xml.NewDecoder(r)
	decoder.CharsetReader = charset.NewReaderLabel
	err := decoder.Decode(&rdfData)
	if err != nil {
		return nil, fmt.Errorf("cannot decode data: %w", err)
	}
	allNews := make([]News, 0, len(rdfData.Items))
	for _, item := range rdfData.Items {
		news, err := NewNews(rdfData.Channel.Title, item.Title, item.Link)
		if err != nil {
			continue
		}
		allNews = append(allNews, news)
	}
	return allNews, nil
}

func ParseAtomResponse(r io.Reader) ([]News, error) {
	type atom struct {
		XMLName xml.Name `xml:"feed"`
		Title   string   `xml:"title"`
		Entries []struct {
			Link struct {
				Href string `xml:"href,attr"`
			} `xml:"link"`
			Title struct {
				Text string `xml:",chardata"`
			} `xml:"title"`
		} `xml:"entry"`
	}
	atomData := atom{}
	decoder := xml.NewDecoder(r)
	decoder.CharsetReader = charset.NewReaderLabel
	err := decoder.Decode(&atomData)
	if err != nil {
		return nil, fmt.Errorf("cannot decode data: %w", err)
	}
	allNews := make([]News, 0, len(atomData.Entries))
	for _, item := range atomData.Entries {
		news, err := NewNews(atomData.Title, item.Title.Text, item.Link.Href)
		if err != nil {
			continue
		}
		allNews = append(allNews, news)
	}
	return allNews, nil
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

type Option func(*MorningPost) error

func FromArgs(args []string) Option {
	return func(m *MorningPost) error {
		fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
		fs.SetOutput(m.Stderr)
		listenPort := fs.Int("p", DefaultListenPort, "Listening port")
		err := fs.Parse(args)
		if err != nil {
			return err
		}
		m.ListenPort = *listenPort
		return nil
	}
}

func WithStderr(w io.Writer) Option {
	return func(m *MorningPost) error {
		if w == nil {
			return errors.New("standard error cannot be nil")
		}
		m.Stderr = w
		return nil
	}
}

func WithStdout(w io.Writer) Option {
	return func(m *MorningPost) error {
		if w == nil {
			return errors.New("standard stdout cannot be nil")
		}
		m.Stdout = w
		return nil
	}
}

func WithClient(c *http.Client) Option {
	return func(m *MorningPost) error {
		if c == nil {
			return errors.New("HTTP client cannot be nil")
		}
		m.Client = c
		return nil
	}
}

func WithFileStore(f *FileStore) Option {
	return func(m *MorningPost) error {
		m.fileStore = f
		return nil
	}
}
