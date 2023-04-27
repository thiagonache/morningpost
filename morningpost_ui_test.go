//go:build ui

package morningpost_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"golang.org/x/net/nettest"
)

func setup(t *testing.T, l net.Listener) func(t *testing.T) {
	m := newMorningPostWithFakeStoreAndNoOutput(t)
	go m.Serve(l)
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

func TestUIFeedIsShownOnTableBodyAfterFillingOutInputURLAndClickingOnButtonCreate(t *testing.T) {
	t.Parallel()
	l, err := nettest.NewLocalListener("tcp")
	if err != nil {
		t.Fatal(err)
	}
	teardown := setup(t, l)
	defer teardown(t)
	want := "https://news.ycombinator.com/rss"
	ctx, cancel := chromedp.NewContext(
		context.Background(),
		//chromedp.WithDebugf(log.Printf),
	)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var got string
	err = chromedp.Run(ctx, submitFeed(`http://`+l.Addr().String()+`/feeds`, `//input[@name="url"]`, `https://news.ycombinator.com/rss`, &got))
	if err != nil {
		t.Fatal(err)
	}
	if want != got {
		t.Fatalf("want feed url %q, got %q", want, got)
	}
}
