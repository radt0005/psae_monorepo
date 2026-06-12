// Command spade-scheduler is the Spade platform's scheduling server.
//
// It owns the in-memory MultiTenantScheduler, persists state to
// PostgreSQL, publishes Job messages to RabbitMQ's spade.jobs queue,
// and consumes WorkerResult messages from spade.results.
//
// Full design: ../../IMPLEMENTATION_PLAN.md.
package main

import "spade_server/cmd/spade-scheduler/app"

func main() { app.Main() }
