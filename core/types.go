package core

import (
	"github.com/google/uuid"
)

type BlockArgs struct {
	Id      uuid.UUID
	Name    string
	Inputs  []uuid.UUID
	outputs []string
	Args    map[string]any
}

type Pipeline struct {
	Id      uuid.UUID
	Version string
	Name    string
	Blocks  []BlockArgs
}

type Block struct {
	Id   uuid.UUID
	Name string
}

type BlockInvocation struct {
	Id         uuid.UUID
	BlockId    string
	PipelineId uuid.UUID
	Inputs     []uuid.UUID
	Arguments  map[string]any
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
	ExecutionStatusMap      = "map"
	ExecutionStatusReduce   = "reduce"
)

type BlockInvocationResult struct {
	Id         uuid.UUID
	PipelineId uuid.UUID
	Status     ExecutionStatus
	Outputs    []string
}

type Worker struct {
	Id          uuid.UUID
	Ip          string
	Description string
}
