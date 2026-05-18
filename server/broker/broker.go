// Package broker is the scheduler-server side of the RabbitMQ transport.
//
// Where the worker reads spade.jobs and writes spade.results, the
// scheduler does the inverse: it writes spade.jobs and reads
// spade.results.  This package provides JobPublisher and ResultConsumer
// adapters that reuse the connection plumbing in spade_runner/broker
// (which already declares the durable queues, owns the reconnect loop,
// and exposes the underlying rmq.Publisher / rmq.Consumer interfaces).
package broker

import (
	"context"
	"encoding/json"

	"core"
	rbroker "spade_runner/broker"

	spade "spade_runner"
)

// JobPublisher writes spade.jobs.  Implementations include a real
// RabbitMQ-backed adapter and an in-memory fake (fake.go).
type JobPublisher interface {
	// Publish serializes the job to JSON, publishes it persistently,
	// and blocks until the broker confirms the message.  Returns the
	// underlying transport error on failure so callers can decide
	// whether to retry, reconnect, or fail.
	Publish(ctx context.Context, job spade.Job) error
	// Close releases the underlying publisher.
	Close(ctx context.Context) error
}

// ResultDelivery is one message pulled off spade.results.
type ResultDelivery struct {
	Result  core.WorkerResult
	RawBody []byte
	Ack     func(ctx context.Context) error
	Nack    func(ctx context.Context, requeue bool) error
}

// ResultConsumer reads spade.results.
type ResultConsumer interface {
	Next(ctx context.Context) (ResultDelivery, error)
	Close(ctx context.Context) error
}

// rbrokerPublisher adapts the runner's underlying ResultPublisher (which
// is also used for job publishing on the server side — same interface,
// different queue) to our spade.Job-typed JobPublisher.
type rbrokerPublisher struct {
	inner rbroker.ResultPublisher
}

// NewJobPublisher wraps an rbroker.ResultPublisher (opened against
// spade.jobs via Conn.NewJobPublisher in the runner package) as a
// scheduler JobPublisher.
func NewJobPublisher(inner rbroker.ResultPublisher) JobPublisher {
	return &rbrokerPublisher{inner: inner}
}

// Publish marshals the job and delegates to the underlying publisher.
func (p *rbrokerPublisher) Publish(ctx context.Context, job spade.Job) error {
	// The runner publisher accepts an `any` and marshals internally.
	return p.inner.Publish(ctx, job)
}

// Close shuts down the underlying publisher.
func (p *rbrokerPublisher) Close(ctx context.Context) error { return p.inner.Close(ctx) }

// rbrokerConsumer adapts the runner's JobConsumer (whose Delivery.Job is
// a spade.Job) to the scheduler's ResultConsumer (whose Delivery.Result
// is a core.WorkerResult).  We re-parse the raw body as a WorkerResult.
type rbrokerConsumer struct {
	inner rbroker.JobConsumer
}

// NewResultConsumer wraps an rbroker.JobConsumer opened against
// spade.results via Conn.NewResultConsumer.
func NewResultConsumer(inner rbroker.JobConsumer) ResultConsumer {
	return &rbrokerConsumer{inner: inner}
}

// Next pulls the next message and decodes it as a WorkerResult.  On
// malformed JSON the returned ResultDelivery carries a zero-valued
// Result; callers can detect that via Result.InvocationID == "".
func (c *rbrokerConsumer) Next(ctx context.Context) (ResultDelivery, error) {
	d, err := c.inner.Next(ctx)
	if err != nil {
		return ResultDelivery{}, err
	}
	var res core.WorkerResult
	_ = json.Unmarshal(d.RawBody, &res)
	return ResultDelivery{
		Result:  res,
		RawBody: d.RawBody,
		Ack:     d.Ack,
		Nack:    d.Nack,
	}, nil
}

// Close releases the underlying consumer.
func (c *rbrokerConsumer) Close(ctx context.Context) error { return c.inner.Close(ctx) }
