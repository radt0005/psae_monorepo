package core

import (
	"fmt"

	"github.com/google/uuid"
)

/*
type Scheduler interface {
	AddPipeline(Pipeline) error
	CancelPipeline(uuid.UUID) error
	Next(uuid.UUID) (BlockInvocation, error)
	AddWorker(Worker) error
	RemoveWorker(uuid.UUID) error
	Update(uuid.UUID, uuid.UUID, ExecutionStatus)
}
*/

/*
Implement a Scheduler for a single pipeline and a single worker (simplest case)
*/
type SinglePipelineScheduler struct {
	Pipeline      Pipeline
	ExecutionPlan ExecutionPlan
	Cancelled     bool
}

func (s *SinglePipelineScheduler) AddPipeline(p Pipeline) error {
	s.Pipeline = p
	s.Cancelled = false
	// build the execution plan
	plan, err := BuildExecutionPlanFromPipeline(p)
	if err != nil {
		fmt.Println("Error Parsing Pipeline")
		return err
	}

	s.ExecutionPlan = plan

	return nil
}

func (s *SinglePipelineScheduler) CancelPipeline(id uuid.UUID) error {
	s.Cancelled = true
	return nil
}

func (s *SinglePipelineScheduler) Update(result BlockInvocationResult) error {
	return nil
}

func (s *SinglePipelineScheduler) Next() (BlockInvocation, error) {
	var invocation BlockInvocation

	return invocation, nil

}

type MultiTenantScheduler struct {
	Pipelines         map[uuid.UUID]Pipeline
	ExecutionPlans    map[uuid.UUID]ExecutionPlan
	Workers           map[uuid.UUID]Worker
	CurrentExecutions map[uuid.UUID]BlockInvocation
}

func (s *MultiTenantScheduler) AddPipeline(p Pipeline) error {
	return nil
}

func (s *MultiTenantScheduler) CancelPipeline(id uuid.UUID) error {

	return nil
}

func (s *MultiTenantScheduler) AddWorker(w Worker) error {
	return nil
}

func (s *MultiTenantScheduler) RemoveWorker(id uuid.UUID) error {
	return nil
}

func (s *MultiTenantScheduler) Update(workerId uuid.UUID, invocationId uuid.UUID, result BlockInvocationResult) error {
	return nil
}

func (s *MultiTenantScheduler) NextForWorker(workerId uuid.UUID)
