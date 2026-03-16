package core

import (
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
	Pipeline         Pipeline
	ExecutionPlan    ExecutionPlan
	Cancelled        bool
	Mapping          bool
	ExecutableBlocks []BlockInvocation
	CompletedBlocks  map[uuid.UUID]BlockInvocationResult
	PendingBlocks    map[uuid.UUID]BlockInvocation
}

func (s *SinglePipelineScheduler) AddPipeline(p Pipeline) error {
	s.Pipeline = p
	s.Cancelled = false
	// build the execution plan
	plan, err := BuildExecutionPlanFromPipeline(p)
	if err != nil {
		//fmt.Println("Error Parsing Pipeline")
		return err
	}

	s.ExecutionPlan = plan
	for _, item := range p.Blocks {

		invocation := BlockInvocation{
			Id:        item.Id,
			BlockId:   item.Name,
			Inputs:    item.Inputs,
			Arguments: item.Args,
		}

		s.PendingBlocks[item.Id] = invocation

	}

	return nil
}

func (s *SinglePipelineScheduler) CancelPipeline(id uuid.UUID) error {
	s.Cancelled = true
	s.PendingBlocks = map[uuid.UUID]BlockInvocation{}
	return nil
}

func (s *SinglePipelineScheduler) Update(result BlockInvocationResult) error {

	if result.Status == ExecutionStatusError {
		// error executing, going to cancel the execution
		s.CancelPipeline(s.Pipeline.Id)
	}

	if result.Status == ExecutionStatusComplete {
		// Add to the list of executable blocks
		s.CompletedBlocks[result.Id] = result

		for _, value := range s.PendingBlocks {

			executable := true

			for _, item := range value.Inputs {

				_, contains := s.PendingBlocks[item]

				executable = executable && contains
			}

			if executable {
				s.ExecutableBlocks = append(s.ExecutableBlocks, value)
			}
		}

	}

	if result.Status == ExecutionStatusMap {
		s.HandleMap(result.Outputs, result.Id)
	}

	if result.Status == ExecutionStatusReduce {
		s.HandleReduce(result.Outputs)
	}

	return nil
}

func (s *SinglePipelineScheduler) IsReady() bool {
	return len(s.ExecutableBlocks) > 0
}

func (s *SinglePipelineScheduler) Next() (BlockInvocation, error) {
	var invocation BlockInvocation

	if len(s.ExecutableBlocks) > 0 {
		block := s.ExecutableBlocks[0]
		s.ExecutableBlocks = s.ExecutableBlocks[1:]

		return block, nil
	}

	return invocation, nil

}

func (s *SinglePipelineScheduler) HandleMap(targets []string, id uuid.UUID) {

	// extends the current execution plan with the map output

}

func (s *SinglePipelineScheduler) HandleReduce(targets []string) {
	// handles the map reduce

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
