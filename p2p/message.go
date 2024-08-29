package p2p

// Constants representing types of incoming data over the network.
const (
	IncomingMessage = 0x1 // IncomingMessage indicates a standard message being received.
	IncomingStream  = 0x2 // IncomingStream indicates a stream of data being received.
)

// RPC (Remote Procedure Call) represents the data structure used for communication between nodes in the network.
type RPC struct {
	From    string // The address of the node sending the message.
	Payload []byte // The actual data being sent in the message.
	Stream  bool   // A flag indicating if the message is a stream of data.
}
