package main

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

const defaultRootFolderName = "go_network"

func CASPathTransformFunc(key string) PathKey {
	hash := sha1.Sum([]byte(key))
	hashStr := hex.EncodeToString(hash[:])

	blocksize := 5
	sliceLen := len(hashStr) / blocksize
	paths := make([]string, sliceLen)

	for i := 0; i < sliceLen; i++ {
		from, to := i*blocksize, (i*blocksize)+blocksize
		paths[i] = hashStr[from:to]
	}
	return PathKey{
		Pathname: strings.Join(paths, "/"),
		Filename: hashStr,
	}
}

type PathTransformFunc func(string) PathKey

type PathKey struct {
	Pathname string
	Filename string
}

func (p PathKey) Fullpath() string {
	return fmt.Sprintf("%s/%s", p.Pathname, p.Filename)
}

type StoreOpts struct {
	// Root is the folder name of the root, containing all the folder/files of systems
	Root              string
	PathTransformFunc PathTransformFunc
}

var DefaultPathTransformFunc = func(key string) PathKey {
	return PathKey{
		Pathname: key,
		Filename: key,
	}
}

type Store struct {
	StoreOpts
}

func NewStore(opts StoreOpts) *Store {
	// If transformFunc not defined
	if opts.PathTransformFunc == nil {
		opts.PathTransformFunc = DefaultPathTransformFunc
	}
	if len(opts.Root) == 0 {
		opts.Root = defaultRootFolderName
	}
	return &Store{
		StoreOpts: opts,
	}
}

func (s *Store) Clear() error {
	return os.RemoveAll(s.Root)
}

// Returns first filepathname
func (p PathKey) FirstPathName() string {
	paths := strings.Split(p.Pathname, "/")
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}

// Check whether key is present or not
func (s *Store) Has(id, key string) bool {
	PathKey := s.PathTransformFunc(key)
	FullPathWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, PathKey.Fullpath())
	_, err := os.Stat(FullPathWithRoot)
	return !errors.Is(err, os.ErrNotExist)
}

// Delete the file contents and its children
func (s *Store) Delete(id, key string) error {
	PathKey := s.PathTransformFunc(key)
	defer func() {
		log.Printf("deleted [%s] from disk", PathKey.Filename)
	}()
	FirstPathNameWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, PathKey.FirstPathName())
	return os.RemoveAll(FirstPathNameWithRoot)
}

// Return the file directly: Without Streaming....
func (s *Store) Read(id string, key string) (int64, io.Reader, error) {
	return s.readStream(id, key)
}

func (s *Store) readStream(id string, key string) (int64, io.Reader, error) {
	PathKey := s.PathTransformFunc(key)
	PathKeyWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, PathKey.Fullpath())

	file, err := os.Open(PathKeyWithRoot)
	if err != nil {
		return 0, nil, err
	}
	filestats, err := file.Stat()
	if err != nil {
		return 0, nil, err
	}
	return filestats.Size(), file, nil
}

func (s *Store) Write(id string, key string, r io.Reader) (int64, error) {
	return s.writeStream(id, key, r)
}

// Write into disk in Decrypted mode
func (s *Store) WriteDecrypt(enckey []byte, id string, key string, r io.Reader) (int64, error) {
	f, err := s.openFileforWriting(id, key)
	if err != nil {
		return 0, err
	}
	n, err := copyDecrypt(enckey, r, f)
	return int64(n), err
}

// This will open the file for writing purpose
func (s *Store) openFileforWriting(id string, key string) (*os.File, error) {
	pathKey := s.PathTransformFunc(key)
	pathnameWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, pathKey.Pathname)
	if err := os.MkdirAll(pathnameWithRoot, os.ModePerm); err != nil {
		return nil, err
	}
	pathandFilenameWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, pathKey.Fullpath())
	return os.Create(pathandFilenameWithRoot)
}

func (s *Store) writeStream(id string, key string, r io.Reader) (int64, error) {
	f, err := s.openFileforWriting(id, key)
	if err != nil {
		return 0, err
	}
	// Close the file else Delete will not able to access it.
	defer f.Close()
	return io.Copy(f, r)
}
