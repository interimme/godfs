package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"io"
)

// generateID creates a unique identifier using random bytes.
// It returns the ID as a hex-encoded string.
func generateID() string {
	buf := make([]byte, 32)        // Create a buffer for 32 random bytes
	io.ReadFull(rand.Reader, buf)  // Fill the buffer with random bytes
	return hex.EncodeToString(buf) // Convert the buffer to a hex string and return
}

// hashkey generates an MD5 hash of the given key and returns it as a hex-encoded string.
// This is typically used for generating a hash from a key to create a unique identifier.
func hashkey(key string) string {
	hash := md5.Sum([]byte(key))       // Compute the MD5 hash of the key
	return hex.EncodeToString(hash[:]) // Convert the hash to a hex string and return
}

// newEncryptKey generates a new 256-bit (32-byte) encryption key using random bytes.
// This key can be used for AES encryption/decryption.
func newEncryptKey() []byte {
	keyBuff := make([]byte, 32)       // Create a buffer for 32 random bytes
	io.ReadFull(rand.Reader, keyBuff) // Fill the buffer with random bytes
	return keyBuff                    // Return the generated key
}

// copyStream encrypts or decrypts data from the source (src) and writes it to the destination (dst).
// The function uses a cipher.Stream to perform the encryption/decryption, and returns the total
// number of bytes written or an error if one occurs.
func copyStream(stream cipher.Stream, blockSize int, src io.Reader, dst io.Writer) (int, error) {
	var (
		buf = make([]byte, 32*1024) // Buffer for reading data in chunks (32KB)
		nw  = blockSize             // Initialize the number of bytes written with the block size
	)

	// Read from the source and write to the destination in a loop until the end of the source is reached
	for {
		n, err := src.Read(buf) // Read up to 32KB of data into the buffer
		if n > 0 {
			stream.XORKeyStream(buf, buf[:n]) // Encrypt/decrypt the data in the buffer using the cipher stream
			nn, err := dst.Write(buf[:n])     // Write the processed data to the destination
			if err != nil {
				return 0, err // Return if an error occurs during writing
			}
			nw += nn // Update the number of bytes written
		}
		if err == io.EOF {
			break // Exit the loop when the end of the source is reached
		}
		if err != nil {
			return 0, err // Return if an error occurs during reading
		}
	}
	return nw, nil // Return the total number of bytes written
}

// copyDecrypt decrypts data from the source (src) and writes the decrypted data to the destination (dst).
// It uses the provided AES encryption key and CTR mode for decryption.
func copyDecrypt(key []byte, src io.Reader, dst io.Writer) (int, error) {
	block, err := aes.NewCipher(key) // Create a new AES cipher with the given key
	if err != nil {
		return 0, err // Return if an error occurs during cipher creation
	}
	// Read the Initialization Vector (IV) from the source. The IV must be the same size as the AES block size.
	iv := make([]byte, block.BlockSize())
	if _, err := src.Read(iv); err != nil {
		return 0, err // Return if an error occurs during IV reading
	}

	// Create a new CTR stream cipher for decryption with the AES block and the IV
	stream := cipher.NewCTR(block, iv)
	return copyStream(stream, block.BlockSize(), src, dst) // Decrypt the data and write it to the destination
}

// copyEncrypt encrypts data from the source (src) and writes the encrypted data to the destination (dst).
// It uses the provided AES encryption key and CTR mode for encryption.
func copyEncrypt(key []byte, src io.Reader, dst io.Writer) (int, error) {
	block, err := aes.NewCipher(key) // Create a new AES cipher with the given key
	if err != nil {
		return 0, err // Return if an error occurs during cipher creation
	}

	// Generate a random Initialization Vector (IV) for the encryption. The IV must be the same size as the AES block size.
	iv := make([]byte, block.BlockSize())
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return 0, err // Return if an error occurs during IV generation
	}

	// Prepend the IV to the beginning of the encrypted output
	if _, err := dst.Write(iv); err != nil {
		return 0, err // Return if an error occurs during IV writing
	}

	// Create a new CTR stream cipher for encryption with the AES block and the IV
	stream := cipher.NewCTR(block, iv)
	return copyStream(stream, block.BlockSize(), src, dst) // Encrypt the data and write it to the destination
}
