package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/interimme/godfs/p2p"
)

// makeServer initializes a new FileServer instance with the specified
// listening address and optional bootstrap nodes for connecting to an
// existing network.
func makeServer(listenAddr string, nodes ...string) *FileServer {
	// Create transport options for the TCP network communication, including
	// address to listen on, handshake function, and decoder for incoming messages.
	tcptransportOpts := p2p.TCPTransportOpts{
		ListenAddr:    listenAddr,
		HandshakeFunc: p2p.NOPHandshakeFunc, // No-op handshake function, can be customized
		Decoder:       p2p.DefaultDecoder{}, // Default decoder for message decoding
	}

	// Initialize a new TCP transport with the specified options.
	tcptransport := p2p.NewTCPTransport(tcptransportOpts)

	// Define file server options, including a unique server ID, encryption key,
	// storage root directory, path transformation function, transport method,
	// and any bootstrap nodes for joining an existing network.
	FileServerOpts := FileServerOpts{
		ID:                generateID(),                                 // Generates a unique ID for the server
		EncyptionKey:      newEncryptKey(),                              // Generates a new encryption key for secure storage
		StorageRoot:       "dir_" + strings.TrimPrefix(listenAddr, ":"), // Sets the storage root directory based on the port number
		PathTransformFunc: CASPathTransformFunc,                         // Function to transform file paths based on content-addressable storage (CAS)
		Transport:         tcptransport,                                 // The TCP transport layer for P2P communication
		BootstrapNodes:    nodes,                                        // List of nodes to bootstrap the P2P network
	}

	// Create a new FileServer instance with the specified options.
	s := NewFileServer(FileServerOpts)

	// Set the OnPeer callback to handle new peers connecting to the server.
	tcptransport.OnPeer = s.OnPeer

	// Return the initialized FileServer instance.
	return s
}

func main() {
	// Initialize two file servers, s1 and s2. s1 listens on port 3000 with no bootstrap nodes.
	// s2 listens on port 4000 and connects to s1 as a bootstrap node.
	s1 := makeServer(":3000", "")
	s2 := makeServer(":4000", ":3000")

	// Start server s1 in a separate goroutine to handle network communication asynchronously.
	go func() {
		log.Fatal(s1.Start()) // Start the server and log any fatal errors
	}()

	// Allow time for s1 to start before starting s2.
	time.Sleep(2 * time.Second)

	// Start server s2 asynchronously after s1 has started.
	go s2.Start()

	// Wait to ensure both servers are fully operational before proceeding with data operations.
	time.Sleep(2 * time.Second)

	// Store and retrieve a test file to verify the distributed file system's functionality.
	for i := 0; i < 1; i++ {
		key := fmt.Sprintf("picture_%d.png", i) // Generate a key for the file to be stored

		// Create a reader containing the data to be stored.
		data := bytes.NewReader([]byte("Big Data files are stored here."))
		s2.Store(key, data) // Store the data on server s2

		// Delete the file from the local storage of s2 to test if the distributed system
		// can retrieve the file from another server.
		if err := s2.store.Delete(s2.ID, key); err != nil {
			log.Fatal(err)
		}

		// Attempt to retrieve the file from the distributed network after deletion from local storage.
		r, err := s2.Get(key)
		if err != nil {
			log.Fatal(err) // Log any error that occurs during file retrieval
		}

		// Read the retrieved file's content into a byte slice.
		b, err := io.ReadAll(r)
		if err != nil {
			log.Fatal(err)
		}

		// Print the retrieved file's content to verify correct retrieval.
		fmt.Println(string(b))
	}
}
