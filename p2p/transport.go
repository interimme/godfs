package p2p

import "net"

// Peer represents a connection between two nodes in the network.
// It extends the net.Conn interface to include additional functionality
// for sending data and managing stream closure.
type Peer interface {
	net.Conn           // Embeds net.Conn to provide basic network connection methods
	Send([]byte) error // Send sends data over the connection. Returns an error if sending fails.
	CloseStream()      // CloseStream signals that a data stream is complete or should be closed.
}

// Transport is an interface that defines the methods required to manage
// network connections between nodes. It abstracts the underlying connection
// mechanism, allowing for various implementations like TCP, UDP, or WebSocket.
type Transport interface {
	Addr() string           // Addr returns the network address that the transport is listening on.
	Dial(string) error      // Dial connects to a remote node at the specified address.
	ListenAndAccept() error // ListenAndAccept starts the transport, listening for and accepting new connections.
	Consume() <-chan RPC    // Consume returns a channel for receiving incoming RPC messages.
	Close() error           // Close shuts down the transport, stopping it from accepting new connections.
}
