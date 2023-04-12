package morningpost_test

import (
	"errors"
	"io/fs"
	"os"
	"runtime"
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

func TestGetAll_ReturnsProperItemsGivenNotEmptyStore(t *testing.T) {
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

func TestNewFileStore_SetsCorrectPathByDefault(t *testing.T) {
	t.Parallel()
	home := os.Getenv("HOME")
	if home == "" {
		t.Fatal("$HOME is not set")
	}
	pathByOS := map[string]string{
		"darwin": home + "/Library/Application Support/MorningPost/morningpost.db",
	}
	want := pathByOS[runtime.GOOS]
	fileStore, err := morningpost.NewFileStore()
	if err != nil {
		t.Fatal(err)
	}
	got := fileStore.Path
	if want != got {
		t.Fatalf("want filestore path %q, got %q", want, got)
	}
}

func TestNewFileStore_CreatesEmptyDataByDefaultGivenPathWithNoStore(t *testing.T) {
	t.Parallel()
	want := map[uint64]morningpost.Feed{}
	fileStore, err := morningpost.NewFileStore(
		morningpost.WithPath(t.TempDir() + "/store.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	got := fileStore.Data
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestWithPath_SetsFileStorePathGivenString(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	want := tempDir + "/store.db"
	fileStore, err := morningpost.NewFileStore(
		morningpost.WithPath(tempDir + "/store.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	got := fileStore.Path
	if want != got {
		t.Fatalf("want filestore path %q, got %q", want, got)
	}
}

func TestNewFileStore_CreatesDirectoryGivenPathNotExist(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	_, err := morningpost.NewFileStore(
		morningpost.WithPath(tempDir + "/directory/bogus/file.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(tempDir + "/directory/bogus")
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("want path %q to exist but it doesn't", tempDir+"/directory/bogus")
	}
	if !info.IsDir() {
		t.Fatalf("want path %q to be a directory but it is not", tempDir+"/directory/bogus")
	}
	if info.Mode().Perm() != fs.FileMode(0755) {
		t.Fatalf("want path %q permission %v, got %v", tempDir+"/directory/bogus", fs.FileMode(0755), info.Mode().Perm())
	}
}
