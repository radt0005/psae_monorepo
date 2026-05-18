package broker

import (
	"context"
	"encoding/json"
	"sync"

	"core"

	spade "spade_runner"
)

// FakeJobPublisher records every published job in memory.  Used by
// engine and integration tests instead of dialling a real broker.
type FakeJobPublisher struct {
	mu        sync.Mutex
	Published []spade.Job
	// FailWith, if non-nil, is returned from every Publish call.
	FailWith error
	Closed   bool
}

// Publish appends to Published unless FailWith is set.
func (p *FakeJobPublisher) Publish(ctx context.Context, job spade.Job) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.FailWith != nil {
		return p.FailWith
	}
	p.Published = append(p.Published, job)
	return nil
}

// PublishedJobs returns a copy of Published — safe for tests to read
// without racing against further publishes.
func (p *FakeJobPublisher) PublishedJobs() []spade.Job {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]spade.Job, len(p.Published))
	copy(out, p.Published)
	return out
}

// Close marks the publisher closed.
func (p *FakeJobPublisher) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Closed = true
	return nil
}

// Compile-time interface check.
var _ JobPublisher = (*FakeJobPublisher)(nil)

// FakeResultConsumer feeds pre-loaded WorkerResults into the engine.
// Each Next call yields the next result in FIFO order.  When empty,
// Next blocks until either a result is enqueued, the consumer is
// closed, or ctx is cancelled.
type FakeResultConsumer struct {
	mu      sync.Mutex
	pending []core.WorkerResult
	ready   chan struct{}

	// Settled records "ack" / "nack-discard" / "nack-requeue" for each
	// delivery returned, in the order they were settled.
	Settled []string

	// BadJSONNext, when set, causes the next Next call to return a
	// zero-valued Result with RawBody populated by the configured
	// bytes — exercising the malformed-payload code path.
	BadJSONNext []byte

	closed bool
}

// NewFakeResultConsumer constructs an empty consumer.
func NewFakeResultConsumer() *FakeResultConsumer {
	return &FakeResultConsumer{ready: make(chan struct{}, 16)}
}

// Enqueue adds a result to the FIFO buffer and signals any waiter.
func (c *FakeResultConsumer) Enqueue(r core.WorkerResult) {
	c.mu.Lock()
	c.pending = append(c.pending, r)
	c.mu.Unlock()
	select {
	case c.ready <- struct{}{}:
	default:
	}
}

// Next blocks until a result is available or ctx is cancelled.
func (c *FakeResultConsumer) Next(ctx context.Context) (ResultDelivery, error) {
	for {
		c.mu.Lock()
		if len(c.BadJSONNext) > 0 {
			body := c.BadJSONNext
			c.BadJSONNext = nil
			d := c.buildDeliveryLocked(core.WorkerResult{}, body)
			c.mu.Unlock()
			return d, nil
		}
		if len(c.pending) > 0 {
			r := c.pending[0]
			c.pending = c.pending[1:]
			body, _ := json.Marshal(r)
			d := c.buildDeliveryLocked(r, body)
			c.mu.Unlock()
			return d, nil
		}
		if c.closed {
			c.mu.Unlock()
			return ResultDelivery{}, context.Canceled
		}
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return ResultDelivery{}, ctx.Err()
		case <-c.ready:
		}
	}
}

func (c *FakeResultConsumer) buildDeliveryLocked(r core.WorkerResult, body []byte) ResultDelivery {
	d := ResultDelivery{Result: r, RawBody: body}
	d.Ack = func(ctx context.Context) error {
		c.mu.Lock()
		c.Settled = append(c.Settled, "ack")
		c.mu.Unlock()
		return nil
	}
	d.Nack = func(ctx context.Context, requeue bool) error {
		c.mu.Lock()
		if requeue {
			c.Settled = append(c.Settled, "nack-requeue")
		} else {
			c.Settled = append(c.Settled, "nack-discard")
		}
		c.mu.Unlock()
		return nil
	}
	return d
}

// Close releases the consumer.  Any pending Next call returns Canceled.
func (c *FakeResultConsumer) Close(ctx context.Context) error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	// Wake any waiting Next.
	select {
	case c.ready <- struct{}{}:
	default:
	}
	return nil
}

// Compile-time interface check.
var _ ResultConsumer = (*FakeResultConsumer)(nil)
