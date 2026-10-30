package mqtt

import (
	"bufio"
	"context"
	"io"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// fakeBroker speaks the small part of MQTT 3.1.1 the executor needs: CONNECT,
// PUBLISH, SUBSCRIBE, PINGREQ and DISCONNECT. It pushes `deliver` on every
// topic a client subscribes to.
type fakeBroker struct {
	ln          net.Listener
	deliver     string
	connackCode byte
	silent      bool
	published   chan string
}

func startBroker(t *testing.T, b *fakeBroker) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	b.ln = ln
	b.published = make(chan string, 16)
	t.Cleanup(func() { _ = ln.Close() })
	go b.serve()
	return "tcp://" + ln.Addr().String()
}

func (b *fakeBroker) serve() {
	for {
		c, err := b.ln.Accept()
		if err != nil {
			return
		}
		go b.handle(c)
	}
}

func (b *fakeBroker) handle(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	for {
		first, body, err := readPacket(r)
		if err != nil || b.silent {
			if b.silent && err == nil {
				continue
			}
			return
		}
		if !b.reply(c, first, body) {
			return
		}
	}
}

// readPacket reads one control packet: fixed header byte, remaining length, body.
func readPacket(r *bufio.Reader) (byte, []byte, error) {
	first, err := r.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	n, shift := 0, 0
	for {
		x, err := r.ReadByte()
		if err != nil {
			return 0, nil, err
		}
		n |= int(x&0x7f) << shift
		if x&0x80 == 0 {
			break
		}
		shift += 7
	}
	body := make([]byte, n)
	_, err = io.ReadFull(r, body)
	return first, body, err
}

// reply answers one packet; false ends the connection.
func (b *fakeBroker) reply(c net.Conn, first byte, body []byte) bool {
	switch first >> 4 {
	case 1:
		send(c, []byte{0x20, 0x02, 0x00, b.connackCode})
	case 3:
		b.onPublish(c, first, body)
	case 8:
		b.onSubscribe(c, body)
	case 12:
		send(c, []byte{0xd0, 0x00})
	case 14:
		return false
	}
	return true
}

func (b *fakeBroker) onPublish(c net.Conn, first byte, body []byte) {
	tl := int(body[0])<<8 | int(body[1])
	b.published <- string(body[2 : 2+tl])
	if (first>>1)&3 == 1 {
		id := body[2+tl : 4+tl]
		send(c, []byte{0x40, 0x02, id[0], id[1]})
	}
}

func (b *fakeBroker) onSubscribe(c net.Conn, body []byte) {
	send(c, []byte{0x90, 0x03, body[0], body[1], 0x00})
	if b.deliver == "" {
		return
	}
	tl := int(body[2])<<8 | int(body[3])
	topic := body[4 : 4+tl]
	pkt := []byte{0x30, byte((2 + tl + len(b.deliver)) & 0x7f), body[2], body[3]}
	pkt = append(pkt, topic...)
	pkt = append(pkt, b.deliver...)
	send(c, pkt)
}

// loggerOnce sets venom's package logger once per test run. Calling InitTestLogger in every test rewrites that
// variable while a client goroutine left by an earlier test may still read it, which the race detector reports.
var loggerOnce sync.Once

func initLogger(t *testing.T) { loggerOnce.Do(func() { venom.InitTestLogger(t) }) }

func runMQTT(t *testing.T, step venom.TestStep) Result {
	t.Helper()
	initLogger(t)
	got, err := Executor{}.Run(context.Background(), step)
	require.NoError(t, err)
	res, ok := got.(Result)
	require.True(t, ok)
	return res
}

func TestRun_PublishQoS1(t *testing.T) {
	b := &fakeBroker{}
	addr := startBroker(t, b)
	res := runMQTT(t, venom.TestStep{
		"addrs": addr, "clientType": "publisher", "timeout": 2000,
		"messages": []interface{}{
			map[string]interface{}{"topic": "a/b", "qos": 1, "payload": "hello"},
			map[string]interface{}{"topic": "a/c", "payload": "x"},
		},
	})
	assert.Empty(t, res.Err)
	assert.Equal(t, "a/b", <-b.published)
	assert.Equal(t, "a/c", <-b.published)
}

func TestRun_PublishEmptyTopic(t *testing.T) {
	addr := startBroker(t, &fakeBroker{})
	res := runMQTT(t, venom.TestStep{
		"addrs": addr, "clientType": "publisher", "timeout": 2000,
		"messages": []interface{}{map[string]interface{}{"payload": "x"}},
	})
	assert.Contains(t, res.Err, "mandatory field Topic was empty")
}

func TestRun_SubscribeJSONAndText(t *testing.T) {
	tests := map[string]struct {
		payload string
		kind    string
	}{
		"object": {`{"k":"v"}`, "map"},
		"array":  {`[1,2]`, "array"},
		"text":   {`plain`, "map"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			addr := startBroker(t, &fakeBroker{deliver: tc.payload})
			res := runMQTT(t, venom.TestStep{
				"addrs": addr, "clientType": "subscriber", "timeout": 2000,
				"topics": []string{"t/1"},
			})
			require.Empty(t, res.Err)
			require.Len(t, res.Messages, 1)
			assert.Equal(t, []byte(tc.payload), res.Messages[0])
			assert.Equal(t, []string{"t/1"}, res.Topics)
			assertJSONKind(t, tc.kind, res.MessagesJSON[0])
		})
	}
}

func assertJSONKind(t *testing.T, kind string, v interface{}) {
	t.Helper()
	if kind == "array" {
		assert.IsType(t, []interface{}{}, v)
		return
	}
	assert.IsType(t, map[string]interface{}{}, v)
}

func TestRun_SubscribeNoMessageTimesOut(t *testing.T) {
	addr := startBroker(t, &fakeBroker{})
	res := runMQTT(t, venom.TestStep{
		"addrs": addr, "clientType": "subscriber", "timeout": 200,
		"topics": []string{"t/1"},
	})
	assert.Empty(t, res.Err)
	require.Len(t, res.Messages, 1)
	assert.Empty(t, res.Messages[0])
}

func TestRun_PersistentQueue(t *testing.T) {
	addr := startBroker(t, &fakeBroker{})
	res := runMQTT(t, venom.TestStep{
		"addrs": addr, "clientType": "persistent_queue", "timeout": 2000,
		"clientId": "venom-test", "persistSubscription": true,
		"topics": []string{"t/1", "t/2"},
	})
	assert.Empty(t, res.Err)
}

func TestRun_ConnectionRefusedByBroker(t *testing.T) {
	addr := startBroker(t, &fakeBroker{connackCode: 5})
	for _, ct := range []string{"publisher", "subscriber", "persistent_queue"} {
		res := runMQTT(t, venom.TestStep{
			"addrs": addr, "clientType": ct, "timeout": 2000, "topics": []string{"x"},
		})
		assert.Contains(t, res.Err, "failed to connect to MQTT", ct)
	}
}

func TestRun_ConnectionRefusedPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := "tcp://" + ln.Addr().String()
	require.NoError(t, ln.Close())
	res := runMQTT(t, venom.TestStep{
		"addrs": addr, "clientType": "publisher", "timeout": 2000, "connectTimeout": 500,
	})
	assert.Contains(t, res.Err, "failed to connect to MQTT")
}

func TestRun_SilentBrokerTimesOut(t *testing.T) {
	addr := startBroker(t, &fakeBroker{silent: true})
	res := runMQTT(t, venom.TestStep{
		"addrs": addr, "clientType": "publisher", "timeout": 3000, "connectTimeout": 300,
	})
	assert.Contains(t, res.Err, "failed to connect to MQTT")
}

// A timeout shorter than connectTimeout used to return a nil client with a nil error, and the step panicked (#3301).
func TestRun_SilentBrokerTimeoutBelowConnectTimeout(t *testing.T) {
	addr := startBroker(t, &fakeBroker{silent: true})
	res := runMQTT(t, venom.TestStep{
		"addrs": addr, "clientType": "publisher", "timeout": 300, "connectTimeout": 5000,
	})
	assert.Contains(t, res.Err, "failed to connect to MQTT")
	assert.Contains(t, res.Err, "timeout of 300 ms")
}

func TestRun_CancelledContext(t *testing.T) {
	initLogger(t)
	addr := startBroker(t, &fakeBroker{silent: true})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := Executor{}.Run(ctx, venom.TestStep{
		"addrs": addr, "clientType": "persistent_queue", "timeout": 2000, "connectTimeout": 1000,
		"topics": []string{"a"},
	})
	require.NoError(t, err)
	assert.Contains(t, resultOf(t, got).Err, "cancellation")
}

// send writes p to the client; a failed write drops the connection, which ends the session.
func send(c net.Conn, p []byte) {
	if _, err := c.Write(p); err != nil {
		c.Close()
	}
}

func resultOf(t *testing.T, got interface{}) Result {
	t.Helper()
	res, ok := got.(Result)
	require.True(t, ok)
	return res
}
