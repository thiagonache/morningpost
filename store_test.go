package morningpost_test

import (
	"errors"
	"io/fs"
	"os"
	"runtime"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/thiagonache/morningpost"
	"golang.org/x/exp/maps"
)

type fakeStore map[uint64]morningpost.Feed

func (f fakeStore) GetAll() []morningpost.Feed {
	return maps.Values(f)
}

func (f fakeStore) Add(feed morningpost.Feed) {
	f[0] = feed
}

func (f fakeStore) Delete(id uint64) {
	delete(f, id)
}

func (f fakeStore) Save() error {
	return nil
}

func (f fakeStore) Load() error {
	return nil
}
func TestAdd_PopulatesStoreGivenFeed(t *testing.T) {
	want := []morningpost.Feed{{
		Endpoint: "fake.url",
	}}
	store := fakeStore{}
	store.Add(morningpost.Feed{
		Endpoint: "fake.url",
	})
	got := store.GetAll()
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestGetAll_ReturnsProperItemsGivenPrePoluatedStore(t *testing.T) {
	t.Parallel()
	want := []morningpost.Feed{
		{
			ID:       0,
			Endpoint: "http://fake-http.url",
		},
		{
			ID:       1,
			Endpoint: "https://fake-https.url",
		},
	}
	store := fakeStore{
		0: morningpost.Feed{
			ID:       0,
			Endpoint: "http://fake-http.url",
		},
		1: morningpost.Feed{
			ID:       1,
			Endpoint: "https://fake-https.url",
		},
	}
	got := store.GetAll()
	sort.Slice(got, func(i, j int) bool { return got[i].Endpoint < got[j].Endpoint })
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestSave_PersistsDataToStore(t *testing.T) {
	t.Parallel()
	want := []morningpost.Feed{
		{
			ID:       3301497760025237570,
			Endpoint: "http://fake-http.url",
		},
		{
			ID:       9746313582359217228,
			Endpoint: "https://fake-https.url",
		},
	}
	tempDir := t.TempDir()
	fileStore, err := morningpost.NewFileStore(
		morningpost.WithFileStorePath(tempDir + "/store.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	fileStore.Add(morningpost.Feed{
		Endpoint: "http://fake-http.url",
	})
	fileStore.Add(morningpost.Feed{
		Endpoint: "https://fake-https.url",
	})
	err = fileStore.Save()
	if err != nil {
		t.Fatal(err)
	}
	fileStore2, err := morningpost.NewFileStore(
		morningpost.WithFileStorePath(tempDir + "/store.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	got := fileStore2.GetAll()
	sort.Slice(got, func(i, j int) bool { return got[i].Endpoint < got[j].Endpoint })
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestDelete_RemovesFeedFromStore(t *testing.T) {
	t.Parallel()
	want := []morningpost.Feed{
		{
			ID:       1,
			Endpoint: "https://fake-https.url",
		},
	}
	store := fakeStore{
		0: morningpost.Feed{
			ID:       0,
			Endpoint: "http://fake-http.url",
		},
		1: morningpost.Feed{
			ID:       1,
			Endpoint: "https://fake-https.url",
		},
	}
	store.Delete(0)
	got := store.GetAll()
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestNewFileStore_SetsDefaultPath(t *testing.T) {
	t.Parallel()
	fileStorePaths := map[string]string{
		"darwin": os.ExpandEnv("$HOME") + "/Library/Application Support/MorningPost/morningpost.db",
		"linux":  "/var/lib/MorningPost/morningpost.db",
	}
	want := fileStorePaths[runtime.GOOS]
	fileStore, err := morningpost.NewFileStore()
	if err != nil {
		t.Fatal(err)
	}
	got := fileStore.Path()
	if want != got {
		t.Fatalf("want Path %q, got %q", want, got)
	}
}

func TestNewFileStore_LoadsDataGivenPopulatedStore(t *testing.T) {
	t.Parallel()
	want := []morningpost.Feed{
		{
			Endpoint: "http://fake-http.url",
		},
		{
			Endpoint: "https://fake-https.url",
		},
	}
	fileStore, err := morningpost.NewFileStore(
		morningpost.WithFileStorePath("testdata/golden/filestore.gob"),
	)
	if err != nil {
		t.Fatal(err)
	}
	got := fileStore.GetAll()
	sort.Slice(got, func(i, j int) bool { return got[i].Endpoint < got[j].Endpoint })
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestNewFileStore_CreatesDirectoryGivenPathNotExist(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	_, err := morningpost.NewFileStore(
		morningpost.WithFileStorePath(tempDir + "/directory/bogus/file.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(tempDir + "/directory/bogus")
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("want to exist but it doesn't")
	}
	if !info.IsDir() {
		t.Fatalf("want %q to be a directory but it is not", tempDir+"/directory/bogus")
	}
	if info.Mode().Perm() != fs.FileMode(0755) {
		t.Fatalf("want permission %v, got %v", fs.FileMode(0755), info.Mode().Perm())
	}

}

func TestWithFileStorePath_SetsPathGivenString(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	want := tempDir + "/morningpost.db"
	fileStore, err := morningpost.NewFileStore(
		morningpost.WithFileStorePath(tempDir + "/morningpost.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	got := fileStore.Path()
	if want != got {
		t.Fatalf("want Path %q, got %q", want, got)
	}
}
