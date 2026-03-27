package core

func Execute(block BlockInvocation, pipelineDir string) (BlockInvocationResult, error) {

	/*
		load the way to call the block

		configure file system stuff
		1. create directory
		2. collect inputs based on the arguments
		3. Set up access controls
		4. Run as a separate goroutine with limited permissions (keep this thread's permissions intact)
		5. Build the output for the scheduler
	*/

	CreateBlockDirectory(block.Id, pipelineDir)

	go func() int {
		// calls the block in the sandbox
		return 2
	}()

	result := BlockInvocationResult{
		Id:         block.Id,
		PipelineId: block.PipelineId,
	}

	return result, nil
}

func WriteArgs(args BlockArgs) error {

	// encode the args as JSON, write them to disk
	return nil
}
