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

func makeServer(listenAddr string, nodes ...string) *FileServer {
	tcptransportOpts := p2p.TCPTransportOpts{
		ListenAddr:    listenAddr,
		HandshakeFunc: p2p.NOPHandshakeFunc,
		Decoder:       p2p.DefaultDecoder{},
	}

	tcptransport := p2p.NewTCPTransport(tcptransportOpts)
	FileServerOpts := FileServerOpts{
		ID:                generateID(),
		EncyptionKey:      newEncryptKey(),
		StorageRoot:       "dir_" + strings.TrimPrefix(listenAddr, ":"),
		PathTransformFunc: CASPathTransformFunc,
		Transport:         tcptransport,
		BootstrapNodes:    nodes,
	}

	s := NewFileServer(FileServerOpts)
	tcptransport.OnPeer = s.OnPeer
	return s
}

func main() {
	s1 := makeServer(":3000", "")
	s2 := makeServer(":4000", ":3000")

	go func() {
		log.Fatal(s1.Start())
	}()

	time.Sleep(2 * time.Second)
	go s2.Start()
	time.Sleep(2 * time.Second)

	for i := 0; i < 1; i++ {
		key := fmt.Sprintf("picture_%d.png", i)

		data := bytes.NewReader([]byte("Big Data files are stored here."))
		s2.Store(key, data)

		// Delete the key in Current server
		// to test the system
		if err := s2.store.Delete(s2.ID, key); err != nil {
			log.Fatal(err)
		}

		r, err := s2.Get(key)
		if err != nil {
			log.Fatal(err)
		}
		b, err := io.ReadAll(r)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(b))
	}
}
