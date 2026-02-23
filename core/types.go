package core

import (
	"github.com/google/uuid"
)

type BlockArgs struct {
	Id      uuid.UUID
	Inputs  []string
	outputs []string
	Args    map[string]any
}

type Pipeline struct {
	Id      uuid.UUID
	Blocks  []BlockArgs
	Version string
	Name    string
}

type Block struct {
	Id   uuid.UUID
	Name string
}

type BlockInvocation struct {
	Id      uuid.UUID
	BlockId uuid.UUID
}

type ExecutionPlan struct {
	Id uuid.UUID
}

type ExecutionStatus string

const (
	ExecutionStatusAwaiting = "waiting"
	ExecutionStatusPending  = "pending"
	ExecutionStatusRunning  = "running"
	ExecutionStatusComplete = "complete"
	ExecutionStatusError    = "error"
)

type BlockInvocationResult struct {
	Id      uuid.UUID
	Status  ExecutionStatus
	Outputs []string
}

type Worker struct {
	id uuid.UUID
}
