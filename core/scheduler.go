package core

import (
	"errors"
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

type SchedulerStep struct {
	ready bool
	block BlockInvocation
}

/*
Implement a Scheduler for a single pipeline and a single worker (simplest case)
*/
type SinglePipelineScheduler struct {
	Pipeline         Pipeline
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
	//plan, err := BuildExecutionPlanFromPipeline(p)

	for _, item := range p.Blocks {

		invocation := BlockInvocation{
			Id:         item.Id,
			PipelineId: s.Pipeline.Id,
			BlockId:    item.Name,
			Inputs:     item.Inputs,
			Arguments:  item.Args,
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

func (s *SinglePipelineScheduler) Next() (BlockInvocation, bool, error) {
	var invocation BlockInvocation

	if len(s.ExecutableBlocks) > 0 {
		block := s.ExecutableBlocks[0]
		s.ExecutableBlocks = s.ExecutableBlocks[1:]

		return block, false, nil
	}

	if len(s.ExecutableBlocks) == 0 && len(s.PendingBlocks) == 0 {
		return invocation, true, nil
	}

	return invocation, false, nil

}

func (s *SinglePipelineScheduler) HandleMap(targets []string, id uuid.UUID) {

	// extends the current execution plan with the map output

}

func (s *SinglePipelineScheduler) HandleReduce(targets []string) {
	// handles the map reduce

}

type MultiTenantScheduler struct {
	ExecutionQueue    []BlockInvocation
	Pipelines         map[uuid.UUID]Pipeline
	Schedulers        map[uuid.UUID]SinglePipelineScheduler
	ExecutionPlans    map[uuid.UUID]ExecutionPlan
	Workers           map[uuid.UUID]Worker
	CurrentExecutions map[uuid.UUID]BlockInvocation
}

func (s *MultiTenantScheduler) AddPipeline(p Pipeline) error {

	s.Pipelines[p.Id] = p
	s.Schedulers[p.Id] = NewSchedulerForPipeline(p)

	return nil
}

func (s *MultiTenantScheduler) CancelPipeline(id uuid.UUID) error {

	ps, ok := s.Schedulers[id]
	if !ok {
		// should be an error
		return errors.New("Failed to Find Pipeline.  Hash it been created?")
	}

	ps.CancelPipeline(id)

	delete(s.Pipelines, id)

	return nil
}

func (s *MultiTenantScheduler) Update(invocationId uuid.UUID, result BlockInvocationResult) error {

	scheduler, ok := s.Schedulers[result.PipelineId]
	if !ok {
		return errors.New("Could not Find Pipeline")
	}

	err := scheduler.Update(result)
	if err != nil {
		return err
	}

	return nil
}

func (s *MultiTenantScheduler) Next(workerId uuid.UUID) (BlockInvocation, bool, error) {
	/*
		Implemenation of Next Function.  This returns the next block to execute.
	*/

	if len(s.ExecutionQueue) > 0 {
		blockCall := s.ExecutionQueue[0]
		s.ExecutionQueue = s.ExecutionQueue[1:]
		return blockCall, false, nil
	}

	for id, scheduler := range s.Schedulers {

		// check if the scheduler for that pipeline has work
		if scheduler.IsReady() {
			// if there is work, get it
			invocation, done, err := scheduler.Next()

			// if there is an error, just log it an continue
			if err != nil {
				fmt.Printf("Error processing {%d}: {%d}", id, err)
				continue
			} else {

				if !done {
					// no error and the pipeline is not done, so add it to the execution queue
					s.ExecutionQueue = append(s.ExecutionQueue, invocation)
				}

			}
		}

	}

	if len(s.ExecutionQueue) > 0 {
		blockCall := s.ExecutionQueue[0]
		s.ExecutionQueue = s.ExecutionQueue[1:]
		return blockCall, false, nil

	}

	// this is only reachable if all pipelines are completed.  Return an empty invocation and the true flag
	return BlockInvocation{}, true, nil

}

func NewSchedulerForPipeline(pipeline Pipeline) SinglePipelineScheduler {

	// create a default Scheduler
	scheduler := SinglePipelineScheduler{
		Cancelled:        false,
		Mapping:          false,
		ExecutableBlocks: []BlockInvocation{},
		CompletedBlocks:  map[uuid.UUID]BlockInvocationResult{},
		PendingBlocks:    map[uuid.UUID]BlockInvocation{},
	}

	// add the pipeline
	scheduler.AddPipeline(pipeline)

	return scheduler
}
