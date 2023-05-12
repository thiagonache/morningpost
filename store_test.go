package morningpost_test

import "github.com/thiagonache/morningpost"

type fakeStore struct {
	callAddFeed     []string
	callDeleteFeed  []string
	callRecordVisit []string
	feeds           map[string]bool
	visited         map[string]bool
}

func (f *fakeStore) URLVisited(URL string) bool {
	return f.visited[URL]
}

func (f *fakeStore) RecordVisit(URL string) {
	f.callRecordVisit = append(f.callRecordVisit, URL)
}

func (f *fakeStore) AddFeed(feed morningpost.Feed) {
	f.callAddFeed = append(f.callAddFeed, feed.URL)
}

func (f *fakeStore) DeleteFeed(URL string) {
	f.callDeleteFeed = append(f.callDeleteFeed, URL)
}

func (f *fakeStore) FeedExists(URL string) bool {
	return f.feeds[URL]
}

func (f *fakeStore) GetFeeds() []morningpost.Feed {
	feeds := []morningpost.Feed{}
	for k := range f.feeds {
		//feeds = append(feeds, morningpost.Feed{URL: k})
		feeds = []morningpost.Feed{{URL: k}}
	}
	return feeds
}
