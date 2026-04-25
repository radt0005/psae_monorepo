package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"core"
	"spade_runner/broker"
)

// RunLoop drives the consume → execute → publish → ack pipeline for a
// single connected consumer/publisher pair.  Returns when ctx is
// cancelled or a fatal transport error occurs.
//
// Ordering per ../../spec/worker.md §Communication:
//   1. Consume a Job from spade.jobs (unacked).
//   2. Run the block via w.Run.
//   3. If Run returned a block-level result (err == nil), publish it
//      to spade.results and block until the broker confirms.  Only
//      after the confirm arrives do we Ack the original job.
//   4. If Run returned an infrastructure error, Nack-without-requeue
//      so the broker redelivers to another competing consumer (per
//      §Error Handling case 2), and do not publish a result.
func RunLoop(
	ctx context.Context,
	w *Worker,
	consumer broker.JobConsumer,
	publisher broker.ResultPublisher,
	logger *slog.Logger,
) error {
	if logger == nil {
		logger = slog.Default()
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		delivery, err := consumer.Next(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			return fmt.Errorf("consuming job: %w", err)
		}

		// Malformed JSON: Job is zero-valued.  Nack without requeue.
		if delivery.Job.Assignment.InvocationID == "" && len(delivery.RawBody) > 0 {
			logger.Warn("malformed job payload, discarding", "body_bytes", len(delivery.RawBody))
			if nerr := delivery.Nack(ctx, false); nerr != nil {
				logger.Error("nack failed on malformed payload", "err", nerr)
			}
			continue
		}

		result, runErr := w.Run(ctx, delivery.Job)
		if runErr != nil {
			// Infrastructure failure: nack without requeue, do not publish.
			logger.Error("infrastructure failure, not publishing result",
				"invocation_id", delivery.Job.Assignment.InvocationID,
				"err", runErr)
			if nerr := delivery.Nack(ctx, false); nerr != nil {
				logger.Error("nack failed", "err", nerr)
			}
			if errors.Is(runErr, context.Canceled) {
				return runErr
			}
			continue
		}

		// Block-level result (success or failure): publish, then ack.
		if perr := publisher.Publish(ctx, result); perr != nil {
			logger.Error("publish failed, nacking job",
				"invocation_id", delivery.Job.Assignment.InvocationID,
				"err", perr)
			if nerr := delivery.Nack(ctx, false); nerr != nil {
				logger.Error("nack failed", "err", nerr)
			}
			// A publish failure is a transport issue; propagate so the
			// reconnect loop can re-dial.
			return fmt.Errorf("publishing result: %w", perr)
		}

		if aerr := delivery.Ack(ctx); aerr != nil {
			logger.Error("ack failed after publish",
				"invocation_id", delivery.Job.Assignment.InvocationID,
				"err", aerr)
			// The result is already durably enqueued; the job will be
			// redelivered to another worker.  The scheduler is
			// required to be idempotent per the spec.
			return fmt.Errorf("acking job: %w", aerr)
		}

		logger.Info("job completed",
			"invocation_id", delivery.Job.Assignment.InvocationID,
			"block_name", delivery.Job.Assignment.BlockName,
			"status", result.Status,
			"exit_code", result.ExitCode,
		)
		_ = core.ExecutionStatusComplete // keep core import used if status enum is referenced
	}
}
