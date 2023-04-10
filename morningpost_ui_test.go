//go:build ui

package morningpost_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/thiagonache/morningpost"
)

func setup(t *testing.T) func(t *testing.T) {
	m, err := morningpost.New(
		newFileStoreWithBogusPath(t),
		morningpost.WithStderr(io.Discard),
		morningpost.WithStdout(io.Discard),
		morningpost.FromArgs([]string{"-p", "58000"}),
	)
	if err != nil {
		t.Fatal(err)
	}
	go m.Run()
	return func(t *testing.T) {
		err := m.Shutdown()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func submitFeed(urlstr, sel, q string, res *string) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.Navigate(urlstr),
		chromedp.WaitVisible(sel),
		chromedp.SendKeys(sel, q),
		chromedp.Click(`#button-create`, chromedp.NodeVisible),
		chromedp.WaitVisible(`//*[contains(., 'Feed URL')]`),
		chromedp.Text(`(//tbody//th)`, res),
	}
}

func TestFeedPostForm(t *testing.T) {
	t.Parallel()
	teardown := setup(t)
	defer teardown(t)
	want := "https://news.ycombinator.com/rss"
	ctx, cancel := chromedp.NewContext(
		context.Background(),
		//chromedp.WithDebugf(log.Printf),
	)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var got string
	err := chromedp.Run(ctx, submitFeed(`http://localhost:58000/feeds/`, `//input[@name="url"]`, `https://news.ycombinator.com/rss`, &got))
	if err != nil {
		t.Fatal(err)
	}
	if want != got {
		t.Fatalf("want feed url %q, got %q", want, got)
	}
}
