package main

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"github.com/interimme/godfs/p2p"
	"io"
	"log"
	"sync"
	"time"
)

// FileServerOpts holds the configuration options for the FileServer.
// This includes server identification, encryption keys, storage settings,
// network transport, and bootstrap nodes for joining a network.
type FileServerOpts struct {
	ID                string            // Unique identifier for the node
	EncyptionKey      []byte            // Encryption key for secure file storage
	StorageRoot       string            // Root directory for file storage
	PathTransformFunc PathTransformFunc // Function to transform file paths, often used for CAS
	Transport         p2p.Transport     // Network transport layer (e.g., TCP)
	BootstrapNodes    []string          // List of initial nodes to connect to in the network
}

// FileServer represents a node in the distributed file system.
// It handles peer connections, file storage, and communication with other nodes.
type FileServer struct {
	FileServerOpts // Embedding FileServerOpts to inherit its fields

	peerLock sync.Mutex          // Mutex to synchronize access to peers map
	peers    map[string]p2p.Peer // Map of connected peers in the network

	store  *Store        // Local file storage management
	quitch chan struct{} // Channel to signal server shutdown
}

// NewFileServer initializes and returns a new FileServer instance based on the provided options.
func NewFileServer(opts FileServerOpts) *FileServer {
	storeOpts := StoreOpts{
		Root:              opts.StorageRoot,
		PathTransformFunc: opts.PathTransformFunc,
	}
	if len(opts.ID) == 0 {
		opts.ID = generateID() // Generate a unique ID if none is provided
	}
	return &FileServer{
		FileServerOpts: opts,
		store:          NewStore(storeOpts),       // Initialize the store with the given options
		quitch:         make(chan struct{}),       // Initialize the shutdown channel
		peers:          make(map[string]p2p.Peer), // Initialize the peers map
	}
}

// Message is a general-purpose structure for sending various types of messages across the network.
type Message struct {
	Payload any // The actual data being transmitted, could be any type
}

// MessageStoreFile and MessageGetFile are specific message types
// used for storing and retrieving files across the network.
type MessageStoreFile struct {
	ID   string // Node ID where the file is stored
	Key  string // Unique key identifying the file
	Size int64  // Size of the file in bytes
}

type MessageGetFile struct {
	ID  string // Node ID requesting the file
	Key string // Unique key of the file being requested
}

// broadcast sends a message to all connected peers in the network.
func (s *FileServer) broadcast(msg *Message) error {
	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(msg); err != nil {
		return err // Return an error if encoding fails
	}

	fmt.Printf("Starting BroadCast\n")
	for _, peer := range s.peers {
		// Send the initial message to signal an incoming message
		peer.Send([]byte{p2p.IncomingMessage})
		// Send the actual encoded message
		if err := peer.Send(buf.Bytes()); err != nil {
			return err // Return an error if sending fails
		}
	}
	return nil
}

// Get retrieves a file from the local storage or fetches it from the network if not available locally.
func (s *FileServer) Get(key string) (io.Reader, error) {
	if s.store.Has(s.ID, key) {
		// Serve the file from local storage if available
		fmt.Printf("[%s] serving file (%s) from local disk\n", s.Transport.Addr(), key)
		_, r, err := s.store.Read(s.ID, key)
		return r, err
	}

	// Fetch the file from the network if not available locally
	fmt.Printf("[%s] Don't have file (%s) locally, fetching from network\n", s.Transport.Addr(), key)
	msg := Message{
		Payload: MessageGetFile{
			ID:  s.ID,
			Key: hashkey(key), // Hash the key for secure identification
		},
	}

	if err := s.broadcast(&msg); err != nil {
		return nil, err
	}

	time.Sleep(time.Millisecond * 5) // Wait briefly to ensure message propagation

	// Attempt to receive the file from peers in the network
	for _, peer := range s.peers {
		var fileSize int64
		binary.Read(peer, binary.LittleEndian, &fileSize) // Read the file size from the peer
		// Store the received file data into local storage
		n, err := s.store.WriteDecrypt(s.EncyptionKey, s.ID, key, io.LimitReader(peer, fileSize))
		if err != nil {
			return nil, err
		}
		fmt.Printf("[%s] Received %d bytes over the network from [%s]", s.Transport.Addr(), n, peer.RemoteAddr())
		peer.CloseStream() // Close the stream after receiving data
	}
	_, r, err := s.store.Read(s.ID, key) // Read the file from local storage after download
	return r, err
}

// Store saves a file locally and broadcasts it to all known peers in the network.
func (s *FileServer) Store(key string, r io.Reader) error {
	var (
		fileBuffer = new(bytes.Buffer)           // Buffer to hold file data
		tee        = io.TeeReader(r, fileBuffer) // Reader that duplicates data read to both 'r' and 'fileBuffer'
	)

	// Store the file locally
	size, err := s.store.Write(s.ID, key, tee)
	if err != nil {
		return err
	}

	// Create a message to inform peers of the new file
	msg := Message{
		Payload: MessageStoreFile{
			ID:   s.ID,
			Key:  hashkey(key),
			Size: size + 16, // Include padding for encryption metadata
		},
	}

	// Broadcast the message to all peers
	if err := s.broadcast(&msg); err != nil {
		return err
	}

	fmt.Printf("Starting File upload\n")
	time.Sleep(time.Millisecond * 5) // Brief pause to ensure network stability

	// Send the file to all peers
	peers := []io.Writer{}
	for _, peer := range s.peers {
		peers = append(peers, peer)
	}
	mw := io.MultiWriter(peers...)                        // Create a MultiWriter to write to all peers at once
	mw.Write([]byte{p2p.IncomingStream})                  // Send the initial stream message
	n, err := copyEncrypt(s.EncyptionKey, fileBuffer, mw) // Encrypt and send file data
	if err != nil {
		return err
	}

	fmt.Printf("[%s] send and written (%d) bytes to disk\n", s.Transport.Addr(), n)

	return nil
}

// Stop gracefully shuts down the FileServer by signaling the quit channel.
func (s *FileServer) Stop() {
	close(s.quitch)
}

// OnPeer is called when a new peer connects. It adds the peer to the server's list of peers.
func (s *FileServer) OnPeer(p p2p.Peer) error {
	s.peerLock.Lock()
	defer s.peerLock.Unlock()

	s.peers[p.RemoteAddr().String()] = p
	log.Printf("connected with remote %s", p.RemoteAddr().String())
	return nil
}

// loop continuously listens for incoming messages or quit signals to manage the server's lifecycle.
func (s *FileServer) loop() {
	defer func() {
		log.Println("file server stopped due to error or user quit action")
		s.Transport.Close() // Close the transport when the server stops
	}()

	for {
		select {
		case rpc := <-s.Transport.Consume(): // Handle incoming messages from the transport
			fmt.Println("Checking rpc:-------> on port ", s.Transport.Addr())
			var msg Message
			// Decode the incoming message
			if err := gob.NewDecoder(bytes.NewReader(rpc.Payload)).Decode(&msg); err != nil {
				log.Println("decoding err: ", err)
			}

			// Handle the decoded message
			if err := s.handleMessage(rpc.From, &msg); err != nil {
				log.Println("handleMessage Error: ", err)
			}
		case <-s.quitch: // Exit the loop if a quit signal is received
			return
		}
	}
}

// handleMessage processes incoming messages based on their type (store or get file).
func (s *FileServer) handleMessage(from string, msg *Message) error {
	switch v := msg.Payload.(type) {
	case MessageStoreFile:
		return s.handleMessageStoreFile(from, v) // Handle store file message
	case MessageGetFile:
		return s.handleMessageGetFile(from, v) // Handle get file message
	}
	return nil
}

// handleMessageGetFile handles file retrieval requests from other nodes.
func (s *FileServer) handleMessageGetFile(from string, msg MessageGetFile) error {
	if !s.store.Has(msg.ID, msg.Key) {
		return fmt.Errorf("[%s] need to serve file %s but it does not exist on disk", s.Transport.Addr(), msg.Key)
	}
	log.Printf("[%s] serving file (%s) over the network\n", s.Transport.Addr(), msg.Key)

	fileSize, r, err := s.store.Read(msg.ID, msg.Key) // Read the requested file
	if err != nil {
		return err
	}

	// Ensure proper closure of the reader if it implements io.ReadCloser
	if rc, ok := r.(io.ReadCloser); ok {
		fmt.Println("Closing readcloser")
		defer rc.Close()
	}

	peer, ok := s.peers[from]
	if !ok {
		return fmt.Errorf("could not find the [%s] node in network", from)
	}

	// Send the incoming stream to the requesting server
	peer.Send([]byte{p2p.IncomingStream})

	// Send the file size and data to the remote server
	binary.Write(peer, binary.NativeEndian, fileSize)
	n, err := io.Copy(peer, r) // Copy the file data to the peer
	if err != nil {
		return err
	}

	fmt.Printf("[%s] written %d bytes on the network to [%s]\n", s.Transport.Addr(), n, from)
	return nil
}

// handleMessageStoreFile handles incoming requests to store files on this server.
func (s *FileServer) handleMessageStoreFile(from string, msg MessageStoreFile) error {
	peer, ok := s.peers[from]
	if !ok {
		return fmt.Errorf("peer: [%s] not found in peer list", from)
	}
	// Write the file data received from the remote server to local storage
	n, err := s.store.Write(msg.ID, msg.Key, io.LimitReader(peer, msg.Size))
	if err != nil {
		return err
	}
	log.Printf("Remote Server.... written (%d) bytes to disk", n)
	peer.CloseStream() // Close the peer stream after writing data
	return nil
}

// bootstrapNetwork connects to the bootstrap nodes to join an existing network.
func (s *FileServer) bootstrapNetwork() error {
	for _, addr := range s.BootstrapNodes {
		if len(addr) == 0 {
			continue
		}

		// Start a new connection to each bootstrap node in a separate goroutine
		go func(addr string) {
			log.Printf("[%s] Attempting to connect with remote server %s\n", s.Transport.Addr(), addr)
			if err := s.Transport.Dial(addr); err != nil {
				log.Println("dial error: ", err)
			}
		}(addr)

	}
	return nil
}

// Start begins listening for incoming connections and processes them in a loop.
func (s *FileServer) Start() error {
	log.Printf("Starting File Server %s", s.Transport.Addr())
	if err := s.Transport.ListenAndAccept(); err != nil {
		return err
	}
	s.bootstrapNetwork() // Connect to bootstrap nodes after starting
	s.loop()             // Enter the main server loop
	return nil
}

// init registers the custom message types with the gob encoder for network communication.
func init() {
	gob.Register(MessageStoreFile{})
	gob.Register(MessageGetFile{})
}
