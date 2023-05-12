package morningpost

type Store interface {
	AddFeed(feed Feed)
	DeleteFeed(URL string)
	FeedExists(URL string) bool
	GetFeeds() []Feed
	RecordVisit(URL string)
	URLVisited(URL string) bool
}
