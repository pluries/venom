package rabbitmq

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// closedAddr returns a loopback AMQP URI nothing listens on.
func closedAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return "amqp://" + addr
}

func TestNewAndZeroValue(t *testing.T) {
	e := New()
	_, ok := e.(*Executor)
	assert.True(t, ok)
	assert.Equal(t, Result{}, Executor{}.ZeroValueResult())
	assert.Equal(t, "rabbitmq", Name)
}

func TestGetDefaultAssertions(t *testing.T) {
	a := Executor{}.GetDefaultAssertions()
	require.NotNil(t, a)
	assert.Equal(t, []venom.Assertion{"result.error ShouldBeEmpty"}, a.Assertions)
}

func TestRun_Validation(t *testing.T) {
	venom.InitTestLogger(t)
	tests := []struct {
		name string
		step venom.TestStep
		want string
	}{
		{"decode error", venom.TestStep{"messageLimit": "many"}, ""},
		{"missing address", venom.TestStep{"clientType": "publisher"}, "address is mandatory"},
		{"bad client type", venom.TestStep{"addrs": "amqp://127.0.0.1:1", "clientType": "other"}, `clientType "other" must be`},
		{"empty client type", venom.TestStep{"addrs": "amqp://127.0.0.1:1"}, "must be publisher or subscriber or client"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := Executor{}.Run(context.Background(), tt.step)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
			assert.Nil(t, res)
		})
	}
}

func TestRun_ConnectionRefused(t *testing.T) {
	venom.InitTestLogger(t)
	for _, clientType := range []string{"publisher", "subscriber", "client"} {
		t.Run(clientType, func(t *testing.T) {
			res, err := Executor{}.Run(context.Background(), venom.TestStep{
				"addrs":      closedAddr(t),
				"clientType": clientType,
				"qName":      "q",
			})
			require.Error(t, err)
			assert.Nil(t, res)
		})
	}
}

func TestOpenChannel_BadURI(t *testing.T) {
	venom.InitTestLogger(t)
	_, _, err := Executor{Addrs: "http://not-amqp"}.openChannel(context.Background())
	require.Error(t, err)
}

func TestProcessMessage(t *testing.T) {
	venom.InitTestLogger(t)
	tests := []struct {
		name string
		body string
		want interface{}
	}{
		{"array", `[1,2]`, []interface{}{json.Number("1"), json.Number("2")}},
		{"object", `{"a":"b"}`, map[string]interface{}{"a": "b"}},
		{"plain text", `hello`, map[string]interface{}{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, bodyJSON := Executor{}.processMessage(context.Background(), amqp.Delivery{Body: []byte(tt.body)}, true, nil, nil)
			assert.Equal(t, []string{tt.body}, body)
			require.Len(t, bodyJSON, 1)
			assert.Equal(t, tt.want, bodyJSON[0])
		})
	}
}
