package amqp

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	amqp "github.com/Azure/go-amqp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// closedAddr returns a loopback address nothing listens on, by closing a
// listener that was just bound.
func closedAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return "amqp://" + addr
}

// silentAddr returns a loopback listener that accepts connections and never
// speaks, like a broker stuck before its protocol header.
func silentAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			defer c.Close()
		}
	}()
	t.Cleanup(func() {
		l.Close()
		<-done
	})
	return "amqp://" + l.Addr().String()
}

func TestRun_DecodeError(t *testing.T) {
	venom.InitTestLogger(t)
	_, err := Executor{}.Run(context.Background(), venom.TestStep{"messageLimit": "not a number"})
	require.Error(t, err)
}

func TestRun_MissingAddr(t *testing.T) {
	venom.InitTestLogger(t)
	res, err := Executor{}.Run(context.Background(), venom.TestStep{"clientType": "producer"})
	require.EqualError(t, err, "creating session: addr is mandatory")
	assert.Nil(t, res)
}

func TestRun_ConnectionRefused(t *testing.T) {
	venom.InitTestLogger(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := Executor{}.Run(ctx, venom.TestStep{"addr": closedAddr(t), "clientType": "consumer"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating session")
}

func TestRun_SilentBrokerTimesOut(t *testing.T) {
	venom.InitTestLogger(t)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := Executor{}.Run(ctx, venom.TestStep{"addr": silentAddr(t), "clientType": "producer"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating session")
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestPublishMessages_Validation(t *testing.T) {
	tests := []struct {
		name string
		e    Executor
		want string
	}{
		{"no target", Executor{Messages: []string{"a"}}, "targetAddr is manatory"},
		{"no messages", Executor{TargetAddr: "q"}, "messages length must be > 0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// validation happens before the session is touched
			res, err := tt.e.publishMessages(context.Background(), nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
			assert.Nil(t, res)
		})
	}
}

func TestConsumeMessages_Validation(t *testing.T) {
	tests := []struct {
		name string
		e    Executor
		want string
	}{
		{"no source", Executor{MessageLimit: 1}, "sourceAddr is manatory"},
		{"no limit", Executor{SourceAddr: "q"}, "messageLimit must be > 0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := tt.e.consumeMessages(context.Background(), nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
			assert.Nil(t, res)
		})
	}
}

// scriptedReceiver returns a canned message or a canned error per call.
type scriptedReceiver struct {
	body       string
	receiveErr error
	acceptErr  error
}

func (s scriptedReceiver) Receive(_ context.Context, _ *amqp.ReceiveOptions) (*amqp.Message, error) {
	if s.receiveErr != nil {
		return nil, s.receiveErr
	}
	return amqp.NewMessage([]byte(s.body)), nil
}

func (s scriptedReceiver) AcceptMessage(_ context.Context, _ *amqp.Message) error {
	return s.acceptErr
}

func TestConsumeMessage_Outcomes(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name     string
		recv     scriptedReceiver
		wantStr  string
		wantJSON interface{}
		wantErr  error
	}{
		{"json object", scriptedReceiver{body: `{"a":1}`}, `{"a":1}`, map[string]interface{}{"a": json.Number("1")}, nil},
		{"plain text", scriptedReceiver{body: "hello"}, "hello", nil, nil},
		{"receive error", scriptedReceiver{receiveErr: boom}, "", nil, boom},
		{"accept error", scriptedReceiver{body: "x", acceptErr: boom}, "", nil, boom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, j, err := consumeMessage(context.Background(), tt.recv)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantStr, s)
			assert.Equal(t, tt.wantJSON, j)
		})
	}
}
