# Block Caller (Worker)

The Block Caller or Worker process is responsible for turning the output of the scheduler into a block invocation as a subprocess.  This means that it provides a number of functions including security, logging, and execution.

Broadly, the code here is broken into two parts.  There is the core library, and the worker binary.  These worker binaries also handle communication with the server and are responsible for the communication with the scheduling server as well.  

The calling process also makes sure that file inputs are where they need to be by simlinking the block outputs in one directory to the required location in the next blocks inputs folder.  This is all done based on the block schemas and dependencies between blocks. 


## File System
Blocks are called with an invocation ID.  For example, if a block is called with invocation ID "019cf4bc-3695-7985-b3ad-4b3c88a4e04f", then the block would execute with a directory of the same name.  This folder would have four things in it, 
- `019cf4bc-3695-7985-b3ad-4b3c88a4e04f/params.json`: This is the parameters supplied by the user in the user interface.  These are basic arguments for the block. 
- `019cf4bc-3695-7985-b3ad-4b3c88a4e04f/inputs/<parameter_name>/*`: This subdirectory is the inputs for this block.
- `019cf4bc-3695-7985-b3ad-4b3c88a4e04f/outputs/`: This directory holds the outputs for this block.  Files should be saved here.  
- `019cf4bc-3695-7985-b3ad-4b3c88a4e04f/logs/`: Holds the logs for the block


Based on this layout, when the executor has to call a block, there are some preparations to do. First, the main folder needs to be created.  Then the `params.json` file should be written, and the inputs and outputs folders should be created.  The last thing is to create the symbolic links to the inputs.  

Now the executor gets two things to do this job: 
1. The pipeline block specification, and
2. The block Schemas

Now the pipeline looks something like this:
```yml
id: 0197acd7-92a6-7222-b387-2599729a9edc
name: auxdata
input:
    - 0197acd5-b635-7222-b387-06ca527c6f5d
    - 0197acd6-5145-7222-b387-102e9f7e5ef7
    - 0197acd6-d81a-7222-b387-1b98b3640f91
    - 0197acd6-04d4-7222-b387-0d19b7dfb928
args: {}
```

The id is the invocation ID, the name is the name to look up how to execute the block, and the inputs are the invocation IDs that tell it where to look up the outputs from the dependencies.  Lastly, the args key is the arguments for the params.json file.  This iw written by the front-end (or super-users who want to edit by hand) in YAML, but that is parsed before going into the scheduler, so the worker will get all of this in JSON.  

The input field is the invocation IDs for the dependency blocks.  These each have an output folder, and all of the data should be there in those folders.  Now, it's ambiguous which file should go where.  This should be handled in the following way: 
1. The input types in the specified block.schema.json file for that block should match the type of output (the .tif file must go with the raster-typed parameters)
2. The order the blocks are listed (this is how ties should be broken)

This should be unambiguous from the workers point of view.  

## Execution

The actual block calling should depend on the type of block being called. This means that there are case: 
- Python blocks should be called with `uv run <file>`
- R Blocks should be run with `Rscript`
- Binary blocks (Rust, Go, bundled JavaScript) should be run by calling their binaries directly.
- TypeScript blocks should be bundled into single-file executables using Bun, and run as above


## Security

The worker should use `go-landlock` or a similar technology (e.g. `isolate` on Ubuntu) to prevent the blocks from access data outside their working directory.  While it must allow executing some system binaries (e.g. Apache Arrow), it should also use these technolgies to prevent the leaking of sensitive data or system information. 

This security model is meant to maintain data security why keeping the system performant (compared to, say, using containers for the execution of each block).

## Communication

This system should also communicate with the 