package broker

import (
	"context"
	"errors"
	"sync"

	spade "spade_runner"
)

// FakeConsumer is an in-memory JobConsumer for tests.
//
// Use Enqueue to push Jobs that Next will return.  Each Next call
// produces a Delivery whose Ack/Nack callbacks record the outcome
// on the consumer so tests can assert settlement semantics.
type FakeConsumer struct {
	mu      sync.Mutex
	pending []spade.Job

	// Settled collects, in order, each delivery's final disposition:
	// "ack", "nack-requeue", or "nack-discard".  Tests assert on this.
	Settled []string

	// Closed is set true when Close is called.
	Closed bool

	// BadJSONNext, when set, causes the next Next call to deliver
	// an empty Job + the configured raw body, exercising the
	// malformed-payload handling path.
	BadJSONNext []byte
}

// Enqueue adds a job to the fake's FIFO buffer.
func (f *FakeConsumer) Enqueue(j spade.Job) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pending = append(f.pending, j)
}

// Next returns the head of the pending queue.  When empty, blocks on
// ctx.Done() and returns ctx.Err().  Tests either pre-load jobs before
// calling Next, or cancel the context to unblock the consumer.
func (f *FakeConsumer) Next(ctx context.Context) (Delivery, error) {
	for {
		f.mu.Lock()
		if len(f.BadJSONNext) > 0 {
			raw := f.BadJSONNext
			f.BadJSONNext = nil
			d := f.buildDeliveryLocked(spade.Job{}, raw)
			f.mu.Unlock()
			return d, nil
		}
		if len(f.pending) > 0 {
			j := f.pending[0]
			f.pending = f.pending[1:]
			raw, _ := marshalJob(j)
			d := f.buildDeliveryLocked(j, raw)
			f.mu.Unlock()
			return d, nil
		}
		f.mu.Unlock()

		select {
		case <-ctx.Done():
			return Delivery{}, ctx.Err()
		default:
			// Spin with a short yield.  Tests pre-load jobs so this
			// path is rarely hit; a real fake would use a condition
			// variable.  Keep it simple.
		}
	}
}

func (f *FakeConsumer) buildDeliveryLocked(j spade.Job, raw []byte) Delivery {
	d := Delivery{Job: j, RawBody: raw}
	d.Ack = func(ctx context.Context) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.Settled = append(f.Settled, "ack")
		return nil
	}
	d.Nack = func(ctx context.Context, requeue bool) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		if requeue {
			f.Settled = append(f.Settled, "nack-requeue")
		} else {
			f.Settled = append(f.Settled, "nack-discard")
		}
		return nil
	}
	return d
}

// Close marks the fake as closed.  Subsequent Next calls return an error.
func (f *FakeConsumer) Close(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Closed = true
	return nil
}

// FakePublisher is an in-memory ResultPublisher for tests.
// Each Publish call appends the serialized result to Published.
type FakePublisher struct {
	mu        sync.Mutex
	Published [][]byte
	// FailWith, when non-nil, is returned from every Publish.  Used
	// to test infra-failure handling on the publish side.
	FailWith error
	Closed   bool
}

// Publish JSON-encodes the result and records it.
func (p *FakePublisher) Publish(ctx context.Context, result any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.FailWith != nil {
		return p.FailWith
	}
	data, err := marshalResult(result)
	if err != nil {
		return err
	}
	p.Published = append(p.Published, data)
	return nil
}

// PublishedCount returns how many results have been published so far.
func (p *FakePublisher) PublishedCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.Published)
}

// Close marks the publisher as closed.
func (p *FakePublisher) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Closed = true
	return nil
}

// compile-time interface assertions
var (
	_ JobConsumer     = (*FakeConsumer)(nil)
	_ ResultPublisher = (*FakePublisher)(nil)
)

// ErrFakeClosed is returned by a FakeConsumer after Close.
var ErrFakeClosed = errors.New("fake consumer closed")
