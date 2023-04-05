# Documentation

## Add a new Feed

To add new feeds you should type an URL on the [feeds
page](http://localhost:33000/feeds/). Eg.: To add the HackerNews feed you should
type `https://news.ycombinator.com/rss` and hit the button `Create`.

### Auto discovery

You can type the feed URL _if you know it_ as the image above shows, but in case you don't know it **you
can type just the site URL**.
Eg.: if you enter `https://techcrunch.com/`, the system will do auto discovery
in the HTML page and add the feeds `https://techcrunch.com/feed/` and `https://techcrunch.com/comments/feed/`.
