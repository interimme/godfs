package p2p

import "errors"

// ErrInvalidHandshake is an error indicating that a handshake attempt was invalid or failed.
// This error can be returned by a HandshakeFunc to signal a failure in the handshake process.
var ErrInvalidHandshake = errors.New("invalid handshake")

// HandshakeFunc is a function type that represents a handshake operation between peers.
// It takes a Peer as an argument and returns an error if the handshake fails.
type HandshakeFunc func(Peer) error

// NOPHandshakeFunc is a no-operation handshake function that always succeeds.
// It does nothing and returns nil, indicating a successful handshake.
// This can be used as a default or placeholder handshake function where no specific handshake logic is needed.
func NOPHandshakeFunc(Peer) error {
	return nil // Always return nil to indicate success
}
