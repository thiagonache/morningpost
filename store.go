package morningpost

import (
	"encoding/gob"
	"errors"
	"fmt"
	"hash/fnv"
	"os"

	"golang.org/x/exp/maps"
)

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
		return fmt.Errorf("error saving store: %w", err)
	}
	defer file.Close()
	enc := gob.NewEncoder(file)
	return enc.Encode(f.Data)
}

func (f *FileStore) Delete(id uint64) {
	delete(f.Data, id)
}
