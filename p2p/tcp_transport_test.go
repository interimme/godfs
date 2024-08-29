package p2p

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

// TestTCPTransport tests the basic functionality of the TCPTransport, including
// initialization, address assignment, and the ability to listen and accept connections.
func TestTCPTransport(t *testing.T) {
	// Define the options for the TCPTransport, specifying the listening address,
	// handshake function, and decoder to be used.
	opts := TCPTransportOpts{
		ListenAddr:    ":3000",          // Address to listen on for incoming connections
		HandshakeFunc: NOPHandshakeFunc, // No-op handshake function for testing purposes
		Decoder:       DefaultDecoder{}, // Default decoder for handling incoming messages
	}

	// Create a new TCPTransport instance with the specified options
	tr := NewTCPTransport(opts)

	// Assert that the TCPTransport's ListenAddr field is set correctly
	assert.Equal(t, tr.ListenAddr, ":3000")

	// Start listening and accepting connections and assert that no error is returned
	assert.Nil(t, tr.ListenAndAccept())
}
