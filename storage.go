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

// Default folder name for storage root if not specified
const defaultRootFolderName = "go_network"

// CASPathTransformFunc transforms a key into a hashed path using SHA-1,
// which helps in storing files in a content-addressable manner.
func CASPathTransformFunc(key string) PathKey {
	hash := sha1.Sum([]byte(key))          // Generate SHA-1 hash of the key
	hashStr := hex.EncodeToString(hash[:]) // Convert hash to a hexadecimal string

	blocksize := 5 // Number of characters per directory level
	sliceLen := len(hashStr) / blocksize
	paths := make([]string, sliceLen)

	// Split the hash string into directory levels
	for i := 0; i < sliceLen; i++ {
		from, to := i*blocksize, (i*blocksize)+blocksize
		paths[i] = hashStr[from:to]
	}

	// Return a PathKey with the generated directory path and filename
	return PathKey{
		Pathname: strings.Join(paths, "/"),
		Filename: hashStr,
	}
}

// PathTransformFunc is a type for functions that transform a key into a PathKey
type PathTransformFunc func(string) PathKey

// PathKey holds the generated path and filename for a stored file
type PathKey struct {
	Pathname string // Directory path generated from the key
	Filename string // Filename generated from the key
}

// Fullpath returns the full path of the file including both the directory path and filename
func (p PathKey) Fullpath() string {
	return fmt.Sprintf("%s/%s", p.Pathname, p.Filename)
}

// StoreOpts holds configuration options for a Store, such as the root directory and path transformation function
type StoreOpts struct {
	Root              string            // Root directory for file storage
	PathTransformFunc PathTransformFunc // Function to transform keys into paths
}

// DefaultPathTransformFunc is the default function that returns the key itself as both the pathname and filename
var DefaultPathTransformFunc = func(key string) PathKey {
	return PathKey{
		Pathname: key,
		Filename: key,
	}
}

// Store represents the storage layer for files, using the specified options for configuration
type Store struct {
	StoreOpts // Embedding StoreOpts to inherit its fields
}

// NewStore initializes a new Store with the given options.
// If no path transform function or root directory is specified, defaults are used.
func NewStore(opts StoreOpts) *Store {
	// Set default path transform function if not provided
	if opts.PathTransformFunc == nil {
		opts.PathTransformFunc = DefaultPathTransformFunc
	}
	// Set default root directory if not provided
	if len(opts.Root) == 0 {
		opts.Root = defaultRootFolderName
	}
	return &Store{
		StoreOpts: opts,
	}
}

// Clear deletes all files and directories within the store's root directory
func (s *Store) Clear() error {
	return os.RemoveAll(s.Root)
}

// FirstPathName returns the first segment of the directory path generated from the key
func (p PathKey) FirstPathName() string {
	paths := strings.Split(p.Pathname, "/")
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}

// Has checks if a file identified by the key exists in the store for a specific ID
func (s *Store) Has(id, key string) bool {
	PathKey := s.PathTransformFunc(key)                                         // Transform the key to get its path
	FullPathWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, PathKey.Fullpath()) // Construct the full file path
	_, err := os.Stat(FullPathWithRoot)
	return !errors.Is(err, os.ErrNotExist) // Return true if the file exists
}

// Delete removes the file and its parent directory from the store for a specific ID
func (s *Store) Delete(id, key string) error {
	PathKey := s.PathTransformFunc(key) // Transform the key to get its path
	defer func() {
		log.Printf("deleted [%s] from disk", PathKey.Filename) // Log the deletion
	}()
	FirstPathNameWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, PathKey.FirstPathName()) // Get the top-level directory path
	return os.RemoveAll(FirstPathNameWithRoot)                                            // Delete the directory and its contents
}

// Read returns a reader for the file identified by the key in the store for a specific ID
func (s *Store) Read(id string, key string) (int64, io.Reader, error) {
	return s.readStream(id, key)
}

// readStream opens the file for reading and returns its size and reader
func (s *Store) readStream(id string, key string) (int64, io.Reader, error) {
	PathKey := s.PathTransformFunc(key)                                        // Transform the key to get its path
	PathKeyWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, PathKey.Fullpath()) // Construct the full file path

	file, err := os.Open(PathKeyWithRoot) // Open the file for reading
	if err != nil {
		return 0, nil, err
	}
	filestats, err := file.Stat() // Get file statistics
	if err != nil {
		return 0, nil, err
	}
	return filestats.Size(), file, nil // Return file size and reader
}

// Write saves data from the reader to a file identified by the key in the store for a specific ID
func (s *Store) Write(id string, key string, r io.Reader) (int64, error) {
	return s.writeStream(id, key, r)
}

// WriteDecrypt writes data to disk in decrypted mode using the specified encryption key
func (s *Store) WriteDecrypt(enckey []byte, id string, key string, r io.Reader) (int64, error) {
	f, err := s.openFileforWriting(id, key) // Open the file for writing
	if err != nil {
		return 0, err
	}
	n, err := copyDecrypt(enckey, r, f) // Decrypt and write the data
	return int64(n), err
}

// openFileforWriting creates the necessary directories and opens a file for writing
func (s *Store) openFileforWriting(id string, key string) (*os.File, error) {
	pathKey := s.PathTransformFunc(key)                                       // Transform the key to get its path
	pathnameWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, pathKey.Pathname) // Construct the directory path

	// Create the directory path if it doesn't exist
	if err := os.MkdirAll(pathnameWithRoot, os.ModePerm); err != nil {
		return nil, err
	}

	pathandFilenameWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, pathKey.Fullpath()) // Construct the full file path
	return os.Create(pathandFilenameWithRoot)                                          // Create and open the file for writing
}

// writeStream writes data from the reader to a file, creating necessary directories as needed
func (s *Store) writeStream(id string, key string, r io.Reader) (int64, error) {
	f, err := s.openFileforWriting(id, key) // Open the file for writing
	if err != nil {
		return 0, err
	}
	defer f.Close()      // Ensure the file is closed after writing
	return io.Copy(f, r) // Copy the data from the reader to the file
}
