// Package broker provides the RabbitMQ transport layer for the Spade worker.
//
// The Spade worker consumes jobs from a durable AMQP queue (spade.jobs) and
// publishes results to a second durable queue (spade.results).  This package
// wraps the github.com/rabbitmq/rabbitmq-amqp-go-client client with two small
// interfaces (JobConsumer, ResultPublisher) so callers — chiefly the
// spade-worker binary — can stay agnostic to AMQP specifics and so tests can
// substitute in-memory fakes.
//
// See ../../spec/worker.md §Communication for the wire-level contract:
// durable queues, persistent messages, prefetch count of 1 per consumer,
// JSON message bodies, and ack-after-publish semantics.
package broker

import (
	"context"
	"encoding/json"

	spade "spade_runner"
)

// Queue names are fixed by the Spade spec.
const (
	QueueJobs    = "spade.jobs"
	QueueResults = "spade.results"
)

// Delivery represents one message pulled off the job queue.  The
// Ack / Nack callbacks settle the message; the worker must call
// exactly one of them, and must delay Ack until after the result
// has been durably published.
type Delivery struct {
	Job     spade.Job
	RawBody []byte
	// Ack settles the message as accepted, removing it from the queue.
	Ack func(ctx context.Context) error
	// Nack settles the message as rejected.  If requeue is true, the
	// broker puts it back on the queue; if false, the broker discards
	// (or routes to a dead-letter queue if configured).  The worker
	// uses requeue=false so the broker's redelivery timeout reaches
	// another competing consumer per ../../spec/worker.md §Error Handling.
	Nack func(ctx context.Context, requeue bool) error
}

// JobConsumer is the worker-side view of a pull from spade.jobs.
// Implementations include an AMQP client and an in-memory fake.
type JobConsumer interface {
	// Next blocks until the next message arrives or ctx is cancelled.
	// A cancelled context returns the context error; a transport
	// error surfaces as-is.  Malformed JSON bodies are returned with
	// RawBody populated and Job zero-valued so callers can Nack.
	Next(ctx context.Context) (Delivery, error)
	Close(ctx context.Context) error
}

// ResultPublisher is the worker-side view of a send to spade.results.
type ResultPublisher interface {
	// Publish sends a result to spade.results.  It must not return
	// until the broker has durably accepted the message (publisher
	// confirms), so the caller can safely ack the originating job
	// immediately after Publish returns.
	Publish(ctx context.Context, result any) error
	Close(ctx context.Context) error
}

// marshalJob serializes a spade.Job to JSON.  Exposed for tests.
func marshalJob(j spade.Job) ([]byte, error) { return json.Marshal(j) }

// unmarshalJob deserializes a spade.Job from JSON.  Exposed for tests.
func unmarshalJob(data []byte) (spade.Job, error) {
	var j spade.Job
	err := json.Unmarshal(data, &j)
	return j, err
}

// marshalResult JSON-encodes any result payload.  The worker uses
// core.WorkerResult here but we accept any to avoid a core import
// cycle in the fake publisher tests.
func marshalResult(r any) ([]byte, error) { return json.Marshal(r) }
