package morningpost

import (
	"encoding/gob"
	"errors"
	"hash/fnv"
	"os"
	"path"
	"runtime"
	"syscall"

	"golang.org/x/exp/maps"
)

type Store interface {
	Add(...Feed)
	Delete(uint64)
	GetAll() []Feed
	Save() error
}

type FileStore struct {
	data map[uint64]Feed
	path string
}

func (f *FileStore) Add(feeds ...Feed) {
	h := fnv.New64a()
	for _, feed := range feeds {
		h.Write([]byte(feed.Endpoint))
		feed.ID = h.Sum64()
		f.data[h.Sum64()] = feed
		h.Reset()
	}
}

func (f *FileStore) GetAll() []Feed {
	return maps.Values(f.data)
}

func (f *FileStore) Load() error {
	file, err := os.Open(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	dec := gob.NewDecoder(file)
	return dec.Decode(&f.data)
}

func (f *FileStore) Save() error {
	if _, err := os.Stat(path.Dir(f.path)); os.IsNotExist(err) {
		err := os.MkdirAll(path.Dir(f.path), 0755)
		if err != nil {
			return err
		}
	}
	file, err := os.Create(f.path)
	if err != nil {
		return err
	}
	defer file.Close()
	enc := gob.NewEncoder(file)
	return enc.Encode(f.data)
}

func (f *FileStore) Delete(id uint64) {
	delete(f.data, id)
}

func (f *FileStore) Path() string {
	return f.path
}

func NewFileStore(opts ...FileStoreOption) (*FileStore, error) {
	fileStore := &FileStore{
		data: map[uint64]Feed{},
		path: userStateDir() + "/MorningPost/morningpost.db",
	}
	for _, o := range opts {
		o(fileStore)
	}
	err := fileStore.Load()
	if err != nil {
		return nil, err
	}
	return fileStore, nil
}

func getenv(key string) string {
	v, _ := syscall.Getenv(key)
	return v
}

func userStateDir() string {
	switch runtime.GOOS {
	case "windows":
		dir := getenv("AppData")
		if dir == "" {
			return "./"
		}
		return dir
	case "darwin", "ios":
		dir := getenv("HOME")
		if dir == "" {
			return "./"
		}
		dir += "/Library/Application Support"
		return dir
	default: // Unix
		dir := getenv("XDG_STATE_HOME")
		if dir == "" {
			return "/var/lib"
		}
		return dir
	}
}

type FileStoreOption func(*FileStore)

func WithFileStorePath(path string) FileStoreOption {
	return func(f *FileStore) {
		f.path = path
	}
}
