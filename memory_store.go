package morningpost

type MemoryStore struct {
	URLs  []string
	Feeds []Feed
}

func (m *MemoryStore) RecordVisit(URL string) {
	m.URLs = append(m.URLs, URL)
}

func (m *MemoryStore) URLVisited(URL string) bool {
	for _, u := range m.URLs {
		if URL == u {
			return true
		}
	}
	return false
}

func (m *MemoryStore) AddFeed(feed Feed) {
	m.Feeds = append(m.Feeds, feed)
}

func (m *MemoryStore) DeleteFeed(URL string) {
	feeds := []Feed{}
	for _, f := range m.Feeds {
		if URL == f.URL {
			continue
		}
		feeds = append(feeds, f)
	}
	m.Feeds = feeds
}

func (m *MemoryStore) FeedExists(URL string) bool {
	for _, f := range m.Feeds {
		if URL == f.URL {
			return true
		}
	}
	return false
}

func (m *MemoryStore) GetFeeds() []Feed {
	return nil
}
