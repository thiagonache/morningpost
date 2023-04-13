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
	GetAll() []Feed
	Add(Feed)
	Delete(uint64)
	Save() error
}

type FileStore struct {
	Data map[uint64]Feed
	Path string
}

func (f *FileStore) Add(feed Feed) {
	h := fnv.New64a()
	h.Write([]byte(feed.Endpoint))
	feed.ID = h.Sum64()
	f.Data[h.Sum64()] = feed
}

func (f *FileStore) GetAll() []Feed {
	return maps.Values(f.Data)
}

func (f *FileStore) Load() error {
	file, err := os.Open(f.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	dec := gob.NewDecoder(file)
	return dec.Decode(&f.Data)
}

func (f *FileStore) Save() error {
	file, err := os.Create(f.Path)
	if err != nil {
		return err
	}
	defer file.Close()
	enc := gob.NewEncoder(file)
	return enc.Encode(f.Data)
}

func (f *FileStore) Delete(id uint64) {
	delete(f.Data, id)
}

func NewFileStore(opts ...FileStoreOption) (*FileStore, error) {
	fileStore := &FileStore{
		Data: map[uint64]Feed{},
		Path: userStateDir() + "/MorningPost/morningpost.db",
	}
	for _, o := range opts {
		o(fileStore)
	}
	err := fileStore.Load()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path.Dir(fileStore.Path)); os.IsNotExist(err) {
		err := os.MkdirAll(path.Dir(fileStore.Path), 0755)
		if err != nil {
			return nil, err
		}

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
		f.Path = path
	}
}
