package morningpost

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
)

const TitleMaxSize = 80

type Source interface {
	GetNews() ([]News, error)
}

type News struct {
	Title string
	URL   string
}

func (n News) String() string {
	title := n.Title
	if len(title) > TitleMaxSize {
		title = title[:TitleMaxSize]
	}
	return fmt.Sprintf("%-80s %s", title, n.URL)
}

func GetRSSFeed(url string) ([]News, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response status %q", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return ParseRSSResponse(data)
}

func ParseRSSResponse(input []byte) ([]News, error) {
	type rss struct {
		XMLName xml.Name `xml:"rss"`
		Channel struct {
			Items []struct {
				Title string `xml:"title"`
				Link  string `xml:"link"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	r := rss{}
	err := xml.Unmarshal(input, &r)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal data %q: %w", input, err)
	}
	news := make([]News, len(r.Channel.Items))
	for i, item := range r.Channel.Items {
		news[i] = News{
			Title: item.Title,
			URL:   item.Link,
		}
	}
	return news, err
}
