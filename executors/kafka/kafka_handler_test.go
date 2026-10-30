package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// fakeSession is a consumer group session that records the marks; the methods the handler does not call stay nil.
type fakeSession struct {
	sarama.ConsumerGroupSession
	marks []string
}

func (s *fakeSession) Context() context.Context { return context.Background() }

func (s *fakeSession) MarkMessage(_ *sarama.ConsumerMessage, metadata string) {
	s.marks = append(s.marks, metadata)
}

// fakeClaim hands the handler a closed channel of messages.
type fakeClaim struct {
	sarama.ConsumerGroupClaim
	ch chan *sarama.ConsumerMessage
}

func (c fakeClaim) Messages() <-chan *sarama.ConsumerMessage { return c.ch }

func claimOf(msgs ...*sarama.ConsumerMessage) fakeClaim {
	ch := make(chan *sarama.ConsumerMessage, len(msgs))
	for _, m := range msgs {
		ch <- m
	}
	close(ch)
	return fakeClaim{ch: ch}
}

func keyed(key, value string) *sarama.ConsumerMessage {
	return &sarama.ConsumerMessage{Topic: "t", Key: []byte(key), Value: []byte(value)}
}

func newHandler(limit int) *handler {
	return &handler{messages: []Message{}, messagesJSON: []interface{}{}, messageLimit: limit, done: make(chan struct{})}
}

func TestHandler_SetupCleanup(t *testing.T) {
	h := newHandler(0)
	require.NoError(t, h.Setup(nil))
	require.NoError(t, h.Cleanup(nil))
}

func TestHandler_ConsumeAll(t *testing.T) {
	venom.InitTestLogger(t)
	h := newHandler(0)
	h.markOffset = true
	s := &fakeSession{}
	require.NoError(t, h.ConsumeClaim(s, claimOf(keyed("a", `{"x":1}`), keyed("b", "plain"))))
	require.Len(t, h.messages, 2)
	assert.Equal(t, "plain", h.messages[1].Value)
	assert.Equal(t, []string{"", "delivered", "", "delivered"}, s.marks)
}

func TestHandler_KeyFilter(t *testing.T) {
	venom.InitTestLogger(t)
	h := newHandler(0)
	h.keyFilter = "b"
	require.NoError(t, h.ConsumeClaim(&fakeSession{}, claimOf(keyed("a", "1"), keyed("b", "2"))))
	require.Len(t, h.messages, 1)
	assert.Equal(t, "b", h.messages[0].Key)
}

func TestHandler_MessageLimit(t *testing.T) {
	venom.InitTestLogger(t)
	h := newHandler(2)
	require.NoError(t, h.ConsumeClaim(&fakeSession{}, claimOf(keyed("a", "1"), keyed("b", "2"), keyed("c", "3"))))
	assert.Len(t, h.messages, 2)
	select {
	case <-h.done:
	default:
		t.Fatal("the limit was reached but done is still open")
	}
	// another claim arriving after the limit stops at once
	require.NoError(t, h.ConsumeClaim(&fakeSession{}, claimOf(keyed("d", "4"))))
	assert.Len(t, h.messages, 2)
}

func TestHandler_LimitAlreadyHitBeforeAdding(t *testing.T) {
	venom.InitTestLogger(t)
	h := newHandler(1)
	h.messages = []Message{{Key: "pre"}}
	require.NoError(t, h.ConsumeClaim(&fakeSession{}, claimOf(keyed("a", "1"))))
	assert.Len(t, h.messages, 1)
}

func TestHandler_Avro(t *testing.T) {
	venom.InitTestLogger(t)
	bin, err := Convert2Avro([]byte(testValue), testSchema)
	require.NoError(t, err)
	wire, err := CreateMessage(bin, 3)
	require.NoError(t, err)

	h := newHandler(0)
	h.withAVRO = true
	h.schemaReg = fakeRegistry{}
	require.NoError(t, h.ConsumeClaim(&fakeSession{}, claimOf(keyed("k", string(wire)))))
	require.Len(t, h.messages, 1)
	assert.JSONEq(t, testValue, h.messages[0].Value)

	h = newHandler(0)
	h.withAVRO = true
	h.schemaReg = fakeRegistry{err: errors.New("down")}
	err = h.ConsumeClaim(&fakeSession{}, claimOf(keyed("k", string(wire))))
	require.ErrorContains(t, err, "can't get Schema with ID 3")

	h = newHandler(0)
	h.withAVRO = true
	h.schemaReg = fakeRegistry{}
	garbage, err := CreateMessage([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, 3)
	require.NoError(t, err)
	err = h.ConsumeClaim(&fakeSession{}, claimOf(keyed("k", string(garbage))))
	require.ErrorContains(t, err, "can't get value from Avro message")
}

func TestWaitConsumed(t *testing.T) {
	venom.InitTestLogger(t)
	done, cancel := context.WithCancel(context.Background())
	cancel()
	expired, cancelExpired := context.WithTimeout(context.Background(), 0)
	defer cancelExpired()

	tests := map[string]struct {
		e       Executor
		ctx     context.Context
		err     error
		wantErr string
	}{
		"consumer returned nil":          {Executor{}, context.Background(), nil, ""},
		"consumer failed":                {Executor{}, context.Background(), errors.New("boom"), "error on consume: boom"},
		"waitFor deadline from consumer": {Executor{WaitFor: 1}, context.Background(), context.DeadlineExceeded, ""},
		"canceled context":               {Executor{}, done, nil, "kafka consumed failed"},
		"waitFor deadline on context":    {Executor{WaitFor: 1}, expired, nil, ""},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cherr := make(chan error, 1)
			if tc.err != nil || tc.ctx == context.Background() {
				cherr <- tc.err
			}
			err := tc.e.waitConsumed(tc.ctx, cherr)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestLogConsumeErrors(t *testing.T) {
	venom.InitTestLogger(t)
	errs := make(chan error, 2)
	errs <- context.DeadlineExceeded
	errs <- errors.New("other")
	close(errs)
	g := newBlockingGroup()
	g.errs = errs
	Executor{WaitFor: 1}.logConsumeErrors(context.Background(), g)
}

func TestConsumeTimeout(t *testing.T) {
	assert.Equal(t, int64(5e9), int64(Executor{Timeout: 5}.consumeTimeout()))
	assert.Equal(t, int64(2e9), int64(Executor{Timeout: 5, WaitFor: 2}.consumeTimeout()))
}
