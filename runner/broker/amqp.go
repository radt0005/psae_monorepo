package broker

import (
	"context"
	"errors"
	"fmt"

	"github.com/Azure/go-amqp"
	rmq "github.com/rabbitmq/rabbitmq-amqp-go-client/pkg/rabbitmqamqp"
)

// Conn owns an AMQP connection plus the Spade-specific queue declarations.
// Use Dial to construct one.
type Conn struct {
	env *rmq.Environment
	ac  *rmq.AmqpConnection
}

// Dial opens an AMQP 1.0 connection to the broker and declares the two
// durable queues Spade uses.  Queue declaration is idempotent, so callers
// may invoke this on every reconnect.
func Dial(ctx context.Context, url string) (*Conn, error) {
	env := rmq.NewEnvironment(url, nil)
	ac, err := env.NewConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("opening amqp connection: %w", err)
	}
	c := &Conn{env: env, ac: ac}
	if err := c.ensureQueues(ctx); err != nil {
		_ = c.Close(ctx)
		return nil, err
	}
	return c, nil
}

// ensureQueues declares the two Spade queues as durable quorum queues.
// Safe to call repeatedly — RabbitMQ treats identical declarations as
// no-ops.
func (c *Conn) ensureQueues(ctx context.Context) error {
	mgmt := c.ac.Management()
	for _, q := range []string{QueueJobs, QueueResults} {
		if _, err := mgmt.DeclareQueue(ctx, &rmq.QuorumQueueSpecification{Name: q}); err != nil {
			return fmt.Errorf("declaring queue %s: %w", q, err)
		}
	}
	return nil
}

// Close shuts the connection and the environment down.  Callers should
// pass a context with a short deadline to cap how long a shutdown can take.
func (c *Conn) Close(ctx context.Context) error {
	if c.ac != nil {
		_ = c.ac.Close(ctx)
	}
	if c.env != nil {
		return c.env.CloseConnections(ctx)
	}
	return nil
}

// NewJobConsumer returns a JobConsumer reading from spade.jobs.
// prefetch configures the AMQP 1.0 link credit; the Spade spec mandates
// prefetch == 1 in production so each worker holds exactly one unacked
// job at a time.
func (c *Conn) NewJobConsumer(ctx context.Context, prefetch int32) (JobConsumer, error) {
	if prefetch <= 0 {
		prefetch = 1
	}
	cons, err := c.ac.NewConsumer(ctx, QueueJobs, &rmq.ConsumerOptions{
		InitialCredits: prefetch,
	})
	if err != nil {
		return nil, fmt.Errorf("opening consumer: %w", err)
	}
	return &amqpConsumer{cons: cons}, nil
}

// NewResultPublisher returns a ResultPublisher for spade.results.
func (c *Conn) NewResultPublisher(ctx context.Context) (ResultPublisher, error) {
	pub, err := c.ac.NewPublisher(ctx, &rmq.QueueAddress{Queue: QueueResults}, nil)
	if err != nil {
		return nil, fmt.Errorf("opening publisher: %w", err)
	}
	return &amqpPublisher{pub: pub}, nil
}

// amqpConsumer adapts the rmq.Consumer to the JobConsumer interface.
type amqpConsumer struct {
	cons *rmq.Consumer
}

func (a *amqpConsumer) Next(ctx context.Context) (Delivery, error) {
	dc, err := a.cons.Receive(ctx)
	if err != nil {
		return Delivery{}, err
	}
	msg := dc.Message()
	body := messageBody(msg)
	d := Delivery{RawBody: body}
	// Parse the JSON body; on failure we still hand back a usable
	// Delivery so the worker can Nack without requeue.
	if j, perr := unmarshalJob(body); perr == nil {
		d.Job = j
	}
	d.Ack = func(ctx context.Context) error { return dc.Accept(ctx) }
	d.Nack = func(ctx context.Context, requeue bool) error {
		if requeue {
			return dc.Requeue(ctx)
		}
		return dc.Discard(ctx, &amqp.Error{Condition: "rejected", Description: "rejected by worker"})
	}
	return d, nil
}

func (a *amqpConsumer) Close(ctx context.Context) error {
	if a.cons == nil {
		return nil
	}
	return a.cons.Close(ctx)
}

// amqpPublisher adapts rmq.Publisher to the ResultPublisher interface.
type amqpPublisher struct {
	pub *rmq.Publisher
}

func (p *amqpPublisher) Publish(ctx context.Context, result any) error {
	data, err := marshalResult(result)
	if err != nil {
		return fmt.Errorf("marshaling result: %w", err)
	}
	msg := rmq.NewMessage(data)
	pr, err := p.pub.Publish(ctx, msg)
	if err != nil {
		return fmt.Errorf("publishing result: %w", err)
	}
	switch pr.Outcome.(type) {
	case *rmq.StateAccepted:
		return nil
	case *rmq.StateRejected:
		rej := pr.Outcome.(*rmq.StateRejected)
		if rej.Error != nil {
			return fmt.Errorf("broker rejected result: %s", rej.Error.Error())
		}
		return errors.New("broker rejected result")
	case *rmq.StateReleased:
		return errors.New("broker released result (not routed)")
	default:
		return fmt.Errorf("unexpected delivery state: %T", pr.Outcome)
	}
}

func (p *amqpPublisher) Close(ctx context.Context) error {
	if p.pub == nil {
		return nil
	}
	return p.pub.Close(ctx)
}

// messageBody extracts the bytes payload from an AMQP message.
// The RabbitMQ client uses amqp.Message.Data (a list of byte slices);
// typical messages carry a single-element slice.
func messageBody(msg *amqp.Message) []byte {
	if msg == nil {
		return nil
	}
	if len(msg.Data) > 0 {
		return msg.Data[0]
	}
	if msg.Value != nil {
		if b, ok := msg.Value.([]byte); ok {
			return b
		}
	}
	return nil
}
