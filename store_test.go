package morningpost_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/thiagonache/morningpost"
)

func TestAdd_PopulatesStoreGivenFeed(t *testing.T) {
	want := map[uint64]morningpost.Feed{
		11467468815701994079: {
			Endpoint: "fake.url",
			ID:       11467468815701994079,
		},
	}
	fileStore := &morningpost.FileStore{
		Data: map[uint64]morningpost.Feed{},
		Path: t.TempDir() + "/store.db",
	}
	fileStore.Add(morningpost.Feed{
		Endpoint: "fake.url",
	})
	got := fileStore.Data
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestGetAll_ReturnsProperItemsGivenPrePoluatedStore(t *testing.T) {
	t.Parallel()
	want := []morningpost.Feed{
		{
			Endpoint: "http://fake-http.url",
		},
		{
			Endpoint: "https://fake-https.url",
		},
	}
	fileStore := &morningpost.FileStore{
		Data: map[uint64]morningpost.Feed{},
		Path: t.TempDir() + "/store.db",
	}
	fileStore.Data = map[uint64]morningpost.Feed{
		0: {
			Endpoint: "http://fake-http.url",
		},
		1: {
			Endpoint: "https://fake-https.url",
		},
	}
	got := fileStore.GetAll()
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestLoad_ReturnsExpectedDataGivenEmptyFileStore(t *testing.T) {
	t.Parallel()
	want := map[uint64]morningpost.Feed{}
	fileStore := &morningpost.FileStore{
		Data: map[uint64]morningpost.Feed{},
		Path: t.TempDir() + "/store.db",
	}
	err := fileStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	got := fileStore.Data
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestSave_PersistsDataToStore(t *testing.T) {
	t.Parallel()
	want := map[uint64]morningpost.Feed{
		0: {
			Endpoint: "http://fake-http.url",
		},
		1: {
			Endpoint: "https://fake-https.url",
		},
	}
	tempDir := t.TempDir()
	fileStore := &morningpost.FileStore{
		Data: map[uint64]morningpost.Feed{},
		Path: tempDir + "/store.db",
	}
	fileStore.Data = map[uint64]morningpost.Feed{
		0: {
			Endpoint: "http://fake-http.url",
		},
		1: {
			Endpoint: "https://fake-https.url",
		},
	}
	err := fileStore.Save()
	if err != nil {
		t.Fatal(err)
	}
	fileStore2 := &morningpost.FileStore{
		Data: map[uint64]morningpost.Feed{},
		Path: tempDir + "/store.db",
	}
	fileStore2.Load()
	got := fileStore2.Data
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestDelete_RemovesFeedFromStore(t *testing.T) {
	t.Parallel()
	want := map[uint64]morningpost.Feed{
		1: {
			Endpoint: "https://fake-https.url",
		},
	}
	fileStore := &morningpost.FileStore{
		Data: map[uint64]morningpost.Feed{},
		Path: t.TempDir() + "/store.db",
	}
	fileStore.Data = map[uint64]morningpost.Feed{
		0: {
			Endpoint: "http://fake-http.url",
		},
		1: {
			Endpoint: "https://fake-https.url",
		},
	}
	fileStore.Delete(0)
	got := fileStore.Data
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}
