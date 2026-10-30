package mongo

import (
	"context"
	"encoding/binary"
	"io"
	"math"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/event"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// The mongo driver v2 no longer exports its mock deployment (mtest moved under internal/). mockT is a small
// replacement kept to what these tests used: a real *mongo.Client wired, through a custom dialer, to an in-process
// server that speaks the wire protocol over net.Pipe (no socket, no docker). It answers the handshake itself and
// answers every other command with the next scripted reply; the client's command monitor records the commands sent.

const (
	opReply = 1
	opQuery = 2004
	opMsg   = 2013

	headerLength = 16

	firstBatchKey = "firstBatch"
)

// mockT is one mock client with its scripted replies and the commands it has sent.
type mockT struct {
	*testing.T
	Client *mongo.Client

	mu      sync.Mutex
	replies []bson.D
	started []*event.CommandStartedEvent
}

// Run runs a subtest with a fresh client and an empty reply queue.
func (mt *mockT) Run(name string, f func(mt *mockT)) bool {
	mt.Helper()
	return mt.T.Run(name, func(t *testing.T) {
		sub := &mockT{T: t}
		opts := options.Client().
			SetDialer(pipeDialer{sub}).
			SetHosts([]string{"mock.invalid:27017"}).
			SetDirect(true).
			SetMonitor(&event.CommandMonitor{Started: func(_ context.Context, started *event.CommandStartedEvent) {
				sub.mu.Lock()
				defer sub.mu.Unlock()
				sub.started = append(sub.started, started)
			}})
		client, err := mongo.Connect(opts)
		require.NoError(t, err)
		sub.Client = client
		t.Cleanup(func() {
			if err := client.Disconnect(context.Background()); err != nil {
				t.Logf("disconnecting the mock client: %v", err)
			}
		})
		f(sub)
	})
}

// AddMockResponses queues the replies the next commands receive, in order.
func (mt *mockT) AddMockResponses(replies ...bson.D) {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	mt.replies = append(mt.replies, replies...)
}

// GetAllStartedEvents returns every command sent so far, in order.
func (mt *mockT) GetAllStartedEvents() []*event.CommandStartedEvent {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	return append([]*event.CommandStartedEvent(nil), mt.started...)
}

// ClearEvents forgets the commands sent so far.
func (mt *mockT) ClearEvents() {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	mt.started = nil
}

// next pops the next scripted reply; with none left it answers a plain failure the driver reports as an error.
func (mt *mockT) next() bson.D {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	if len(mt.replies) == 0 {
		return commandErrorResponse(commandError{Code: 1, Name: "InternalError", Message: "no responses remaining"})
	}
	reply := mt.replies[0]
	mt.replies = mt.replies[1:]
	return reply
}

// pipeDialer gives the driver one end of a net.Pipe and serves the other.
type pipeDialer struct{ mt *mockT }

func (d pipeDialer) DialContext(context.Context, string, string) (net.Conn, error) {
	client, server := net.Pipe()
	go d.mt.serve(server)
	return client, nil
}

// serve answers the messages of one connection until it closes.
func (mt *mockT) serve(conn net.Conn) {
	defer conn.Close()
	for {
		message, err := readMessage(conn)
		if err != nil {
			return
		}
		opCode := binary.LittleEndian.Uint32(message[12:16])
		requestID := binary.LittleEndian.Uint32(message[4:8])
		name, ok := commandName(opCode, message[headerLength:])
		if !ok {
			return
		}
		var reply bson.D
		switch name {
		case "hello", "isMaster", "ismaster":
			reply = helloResponse()
		case "endSessions":
			reply = successResponse()
		default:
			reply = mt.next()
		}
		if _, err := conn.Write(replyMessage(opCode, requestID, reply)); err != nil {
			return
		}
	}
}

// readMessage reads one whole wire message, header included.
func readMessage(r io.Reader) ([]byte, error) {
	header := make([]byte, headerLength)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	message := make([]byte, binary.LittleEndian.Uint32(header[0:4]))
	copy(message, header)
	if _, err := io.ReadFull(r, message[headerLength:]); err != nil {
		return nil, err
	}
	return message, nil
}

// commandName returns the first key of the command document of an OP_QUERY or OP_MSG body.
func commandName(opCode uint32, body []byte) (string, bool) {
	var doc []byte
	switch opCode {
	case opQuery:
		// flags, a NUL-terminated namespace, numberToSkip, numberToReturn, then the document.
		end := 4
		for end < len(body) && body[end] != 0 {
			end++
		}
		if end+9 > len(body) {
			return "", false
		}
		doc = body[end+9:]
	case opMsg:
		// flagBits, then a kind-0 section holding the command.
		if len(body) < 6 || body[4] != 0 {
			return "", false
		}
		doc = body[5:]
	default:
		return "", false
	}
	elements, err := bson.Raw(doc).Elements()
	if err != nil || len(elements) == 0 {
		return "", false
	}
	return elements[0].Key(), true
}

// replyMessage frames reply as an OP_REPLY to an OP_QUERY and as an OP_MSG to anything else.
func replyMessage(opCode uint32, requestID uint32, reply bson.D) []byte {
	doc, err := bson.Marshal(reply)
	if err != nil {
		panic(err)
	}
	var payload []byte
	replyCode := uint32(opMsg)
	if opCode == opQuery {
		replyCode = opReply
		payload = make([]byte, 20) // responseFlags, cursorID, startingFrom, numberReturned
		binary.LittleEndian.PutUint32(payload[16:], 1)
	} else {
		payload = []byte{0, 0, 0, 0, 0} // flagBits, kind-0 section
	}
	payload = append(payload, doc...)
	message := make([]byte, headerLength, headerLength+len(payload))
	binary.LittleEndian.PutUint32(message[0:], messageLength(headerLength+len(payload)))
	binary.LittleEndian.PutUint32(message[4:], requestID+1)
	binary.LittleEndian.PutUint32(message[8:], requestID)
	binary.LittleEndian.PutUint32(message[12:], replyCode)
	return append(message, payload...)
}

// messageLength is n as the wire header's uint32; the mock never builds a message anywhere near the limit.
func messageLength(n int) uint32 {
	if n < 0 || int64(n) > math.MaxUint32 {
		panic("mock message too large")
	}
	return uint32(n)
}

// helloResponse is the handshake reply of a standalone server, with the limits the driver reads.
func helloResponse() bson.D {
	return bson.D{
		{Key: "ok", Value: 1},
		{Key: "isWritablePrimary", Value: true},
		{Key: "ismaster", Value: true},
		{Key: "helloOk", Value: true},
		{Key: "maxBsonObjectSize", Value: int32(16777216)},
		{Key: "maxMessageSizeBytes", Value: int32(48000000)},
		{Key: "maxWriteBatchSize", Value: int32(100000)},
		{Key: "logicalSessionTimeoutMinutes", Value: int32(30)},
		{Key: "minWireVersion", Value: int32(0)},
		{Key: "maxWireVersion", Value: int32(21)},
		{Key: "connectionId", Value: int32(1)},
	}
}

// commandError is a command error reply: code, name and message, as the server sends them.
type commandError struct {
	Code    int32
	Message string
	Name    string
}

// writeError is one entry of a write command's writeErrors.
type writeError struct {
	Index   int
	Code    int
	Message string
}

// successResponse is the reply of a command that succeeded, with the given extra fields.
func successResponse(elems ...bson.E) bson.D {
	return append(bson.D{{Key: "ok", Value: 1}}, elems...)
}

// cursorResponse is the reply of a command that opens a cursor; identifier is firstBatchKey or "nextBatch".
func cursorResponse(cursorID int64, ns, identifier string, batch ...bson.D) bson.D {
	documents := bson.A{}
	for _, doc := range batch {
		documents = append(documents, doc)
	}
	return bson.D{
		{Key: "ok", Value: 1},
		{Key: "cursor", Value: bson.D{{Key: "id", Value: cursorID}, {Key: "ns", Value: ns}, {Key: identifier, Value: documents}}},
	}
}

// commandErrorResponse is the reply of a command the server refused.
func commandErrorResponse(ce commandError) bson.D {
	return bson.D{
		{Key: "ok", Value: 0},
		{Key: "code", Value: ce.Code},
		{Key: "errmsg", Value: ce.Message},
		{Key: "codeName", Value: ce.Name},
	}
}

// writeErrorsResponse is the reply of a write command that failed on some of its documents.
func writeErrorsResponse(errs ...writeError) bson.D {
	entries := make(bson.A, len(errs))
	for i, we := range errs {
		entries[i] = bson.D{{Key: "index", Value: we.Index}, {Key: "code", Value: we.Code}, {Key: "errmsg", Value: we.Message}}
	}
	return bson.D{{Key: "ok", Value: 1}, {Key: "writeErrors", Value: entries}}
}
