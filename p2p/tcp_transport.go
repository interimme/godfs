package p2p

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
)

// TCPPeer represents a remote node connected over a TCP connection.
// It holds the underlying network connection, a flag indicating if the connection is outbound,
// and a wait group to manage concurrent operations.
type TCPPeer struct {
	net.Conn                 // The underlying TCP connection for this peer.
	outbound bool            // A flag to indicate if the connection is outbound.
	wg       *sync.WaitGroup // WaitGroup to synchronize the closing of streams.
}

// NewTCPPeer initializes and returns a new TCPPeer instance.
// Takes the TCP connection and a boolean indicating if the connection is outbound.
func NewTCPPeer(conn net.Conn, outbound bool) *TCPPeer {
	return &TCPPeer{
		Conn:     conn,
		outbound: outbound,
		wg:       &sync.WaitGroup{},
	}
}

// CloseStream signals that a stream operation is done by decrementing the wait group counter.
func (p *TCPPeer) CloseStream() {
	p.wg.Done()
}

// Send writes a byte slice to the TCP connection of the peer.
// Returns an error if the write operation fails.
func (p *TCPPeer) Send(b []byte) error {
	_, err := p.Conn.Write(b) // Write data to the connection
	return err                // Return any error encountered
}

// TCPTransportOpts holds the configuration options for TCPTransport.
// This includes the listening address, handshake function, decoder, and peer connection handler.
type TCPTransportOpts struct {
	ListenAddr    string           // The address to listen on for incoming connections.
	HandshakeFunc HandshakeFunc    // Function to perform handshake with a new peer.
	Decoder       Decoder          // Decoder to decode incoming messages.
	OnPeer        func(Peer) error // Function to handle new peer connections.
}

// TCPTransport represents the transport layer using TCP for network communication.
// It manages listening for new connections, handling them, and dispatching RPC messages.
type TCPTransport struct {
	TCPTransportOpts              // Embedding TCPTransportOpts to inherit its fields
	listener         net.Listener // The TCP listener for accepting new connections.
	rpcch            chan RPC     // Channel for delivering incoming RPC messages.
}

// NewTCPTransport initializes and returns a new TCPTransport instance with the given options.
func NewTCPTransport(opts TCPTransportOpts) *TCPTransport {
	return &TCPTransport{
		TCPTransportOpts: opts,
		rpcch:            make(chan RPC, 1024), // Buffered channel for RPC messages
	}
}

// Addr returns the address on which the TCPTransport is listening.
// It implements the Transport interface.
func (t *TCPTransport) Addr() string {
	return t.ListenAddr
}

// Close shuts down the TCP listener and stops accepting new connections.
func (t *TCPTransport) Close() error {
	return t.listener.Close()
}

// Dial connects to a remote peer at the specified address using TCP.
// It implements the Transport interface.
func (t *TCPTransport) Dial(addr string) error {
	conn, err := net.Dial("tcp", addr) // Establish a new TCP connection
	if err != nil {
		return err // Return any error encountered during dialing
	}
	go t.handleConn(conn, true) // Handle the connection asynchronously
	return nil
}

// Consume returns a channel that receives incoming RPC messages from other peers in the network.
// It implements the Transport interface.
func (t *TCPTransport) Consume() <-chan RPC {
	return t.rpcch
}

// ListenAndAccept starts the TCP listener and begins accepting new connections.
func (t *TCPTransport) ListenAndAccept() error {
	var err error
	t.listener, err = net.Listen("tcp", t.ListenAddr) // Start listening on the specified address
	if err != nil {
		return err // Return any error encountered during listening
	}

	go t.startAcceptLoop() // Start a loop to accept new connections asynchronously
	log.Printf("TCP transport listening on port %s\n", t.ListenAddr)
	return nil
}

// startAcceptLoop continuously accepts new TCP connections and handles them.
// It runs in a separate goroutine to handle incoming connections concurrently.
func (t *TCPTransport) startAcceptLoop() {
	for {
		conn, err := t.listener.Accept() // Accept a new connection
		if errors.Is(err, net.ErrClosed) {
			return // Exit if the listener has been closed
		}

		if err != nil {
			fmt.Printf("TCP accept error: %s\n", err) // Log the accept error
			continue
		}
		go t.handleConn(conn, true) // Handle the connection asynchronously
	}
}

// handleConn handles a new incoming or outbound TCP connection.
// It performs the handshake, invokes the peer handler, and starts reading messages from the peer.
func (t *TCPTransport) handleConn(conn net.Conn, outbound bool) {
	var err error

	defer func() {
		fmt.Printf("Dropping peer connection: %s", err) // Log when dropping the connection
		conn.Close()                                    // Ensure the connection is closed on exit
	}()

	peer := NewTCPPeer(conn, outbound) // Initialize a new peer with the connection
	if err := t.HandshakeFunc(peer); err != nil {
		return // Return if the handshake fails
	}

	// If an OnPeer function is defined, call it with the new peer
	if t.OnPeer != nil {
		if err = t.OnPeer(peer); err != nil {
			return // Return if handling the peer fails
		}
	}

	// Read loop to process incoming messages from the peer
	for {
		rpc := RPC{}
		if err := t.Decoder.Decode(conn, &rpc); err != nil {
			fmt.Printf("TCP error: %s\n", err) // Log decoding errors
			return
		}

		rpc.From = conn.RemoteAddr().String() // Set the sender's address in the RPC message
		if rpc.Stream {
			peer.wg.Add(1) // Increment the wait group counter for an incoming stream
			fmt.Printf("[%s] incoming stream, waiting...\n", conn.RemoteAddr().String())
			peer.wg.Wait() // Wait until the stream is closed
			fmt.Printf("[%s] stream closed, resuming read loop\n", conn.RemoteAddr().String())
			continue // Resume the read loop after the stream is closed
		}
		t.rpcch <- rpc // Send the decoded RPC message to the channel
	}
}
