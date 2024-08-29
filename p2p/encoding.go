package p2p

import (
	"encoding/gob"
	"io"
)

// Decoder interface defines a method for decoding data from an io.Reader into an RPC structure.
type Decoder interface {
	Decode(io.Reader, *RPC) error // Decodes data from the reader into the provided RPC object
}

// GODecoder is an implementation of the Decoder interface using Go's gob encoding.
// This decoder can serialize and deserialize data using Go's gob package.
type GODecoder struct{}

// Decode reads and decodes data from the io.Reader into an RPC structure using gob.
// It returns an error if decoding fails.
func (dec GODecoder) Decode(r io.Reader, msg *RPC) error {
	return gob.NewDecoder(r).Decode(msg) // Use gob to decode the reader's content into the msg
}

// DefaultDecoder is another implementation of the Decoder interface that provides
// custom decoding logic for distinguishing between stream and non-stream messages.
type DefaultDecoder struct{}

// Decode reads from the io.Reader and decodes the data into an RPC structure.
// If the data represents a stream, it sets the Stream flag on the RPC object.
func (dec DefaultDecoder) Decode(r io.Reader, msg *RPC) error {
	peekBuf := make([]byte, 1) // Buffer to peek the first byte from the reader
	if _, err := r.Read(peekBuf); err != nil {
		return nil // If reading fails, return nil (no error)
	}
	// Check if the first byte indicates an incoming stream
	stream := peekBuf[0] == IncomingStream
	if stream {
		msg.Stream = true // Mark the message as a stream
		return nil        // Return immediately since no further decoding is needed for streams
	}

	// Buffer for reading the rest of the message
	buf := make([]byte, 1028)
	n, err := r.Read(buf) // Read the remaining data into the buffer
	if err != nil {
		return err // Return any error encountered while reading
	}

	msg.Payload = buf[:n] // Set the payload of the message to the data read
	return nil            // Return nil to indicate successful decoding
}
