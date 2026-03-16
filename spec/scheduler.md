# Scheduler

The scheduler is the heart of the application, in the sense that it is both eseential and central.  

The scheduler is repsonsible for deciding which block will be executed, when, on which worker.  For a single machine, this is relatively simple, but for many concurrent pipelines and many workers this becomes very complex. 

The simple single pipeline scheduler is responsible for two things: 
1. Making sure that the blocks in a pipeline are executed in the correct order, and
2. Handling map and reduce operations for the pipeline (more on that below)


## Order of Execution

The system should maintain an execution order for the blocks in a pipeline based on the dependencies.  This ensures that the dependencies for a block are in place when they are called.  Furthermore, it tracks the execution state of each block, and therefore "knows" which blocks are ready to be executed now, and what the next block to be executed is. 


## Single Instance Scheduler

This is the simplest scheduler, and allows for scheduling a single pipeline. The more complex schedulers are based on multiples of this design.  This one, though, runs a single pipeline.  It is responsible for tracking the correct execution order of the blocks in that pipeline, which blocks have been executed, and whether there are any blocks that can be run now, and which blocks are still remaining to be executed.  It also handles the Map and Reduce operations



## Map and Reduce

The scheduling system is responsible for expansion of map and reduce operations.  These operation allow for applying processes that run on a single file to a directory or collection of files.  This happens in the following way: 
1. A block returns multiple files
2. A map block expands this into one block invocation per output in the previous file, 
3. The runtime applies single-input blocks to these collections, scheduling each block call in parallel on multiple machines
4. A reduce block collects the data back into a single collection or a single file

This is basically equivalent to a "for each" operation. These runtime expansions happen dynamically, and can be composed.  

## Multiple Instance Scheduler

This system is responsible for running many concurrent pipelines on many workers.  We assume that all of the workers share a common file system.  This system maintains a Single Instance Scheduler for each pipeline that is being run, and then assigns block executions to workers.  These workers then handle the execution of the block, and then notify the scheduler of the status (successful execution or error)

This allows for the fair and efficient execution of multiple pipelines across multiple workers.  

