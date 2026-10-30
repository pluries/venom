package kafka

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// blockingGroup is a consumer group whose Consume waits for its context to end, then returns its error, as sarama does
// once the step's deadline has passed.
type blockingGroup struct {
	errs chan error
	once sync.Once
}

func newBlockingGroup() *blockingGroup { return &blockingGroup{errs: make(chan error)} }

func (g *blockingGroup) Consume(ctx context.Context, _ []string, _ sarama.ConsumerGroupHandler) error {
	<-ctx.Done()
	return ctx.Err()
}

func (g *blockingGroup) Errors() <-chan error { return g.errs }

func (g *blockingGroup) Close() error {
	g.once.Do(func() { close(g.errs) })
	return nil
}

func (g *blockingGroup) Pause(map[string][]int32)  {}
func (g *blockingGroup) Resume(map[string][]int32) {}
func (g *blockingGroup) PauseAll()                 {}
func (g *blockingGroup) ResumeAll()                {}

// consumeGoroutines returns the stack of every goroutine still running a closure of Executor.consume.
func consumeGoroutines() []string {
	buf := make([]byte, 1<<16)
	n := runtime.Stack(buf, true)
	for n == len(buf) {
		buf = make([]byte, 2*len(buf))
		n = runtime.Stack(buf, true)
	}
	var left []string
	for _, g := range strings.Split(string(buf[:n]), "\n\n") {
		if strings.Contains(g, "kafka.Executor.consume.func") {
			left = append(left, g)
		}
	}
	return left
}

func TestConsumeExpiredStepLeavesNoGoroutine(t *testing.T) {
	venom.InitTestLogger(t)
	tests := []struct {
		name    string
		exec    Executor
		wantErr bool
	}{
		{name: "WaitFor expires", exec: Executor{Topics: []string{"t"}, WaitFor: 1}},
		{name: "timeout expires", exec: Executor{Topics: []string{"t"}, Timeout: 1}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := newBlockingGroup()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			_, _, err := tt.exec.consume(ctx, group)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, group.Close())

			assert.EventuallyWithT(t, func(c *assert.CollectT) {
				if left := consumeGoroutines(); len(left) > 0 {
					c.Errorf("%d consumer goroutine(s) left:\n%s", len(left), strings.Join(left, "\n\n"))
				}
			}, 3*time.Second, 20*time.Millisecond)
		})
	}
}
