# CLI Specification

This CLI is designed to enable local development of blocks within the system.  This means that it should have the ability to create and run blocks and pipelines, and should be able to publish them using the system.  

## Commands

We are creating a Command Line Interface that uses the following interface:

Usage: `spade [OPTIONS] COMMAND [ARGS]`    

Commands 
- `run`  Runs the specified pipeline
- `check` Validates the specified pipeline file.
- `install` A Command for installing a plugin
- `upload` Uploads a block for security screening and use on the cloud
- `init` Creates the boilerplate for a block collection in the current directory
- `add`  Adds a block to the current collection
- `setup` Set up the PSAE system on the local machine
- `build` Builds all blocks in the collection for upload


## Tools

The system should be able to run pipelines and print various information about the pipelines. The system should be a comprehensive set of development tools for creating blocks for the system.  



## Technology
This CLI should use the following technologies: 
- Go language
- Cobra and Viper for the CLI

The system should connect the Go core package to handle the scheduling, datatypes, etc.  It needs only to run one pipeline at a time, and should use things like

## Installation

The system should be able to install plugins.  This should be done as follows: 
1. Download the files using `git`
2. Run the appropriate install command for the operating system
    1. For rust, this is a `cargo build --release`
    2. For Go, `go build . && go install .`
    3. For Python, `uv sync`
    4. For R, there should be a setup.R file.  We should run it with `Rscript`.
    5. For bun, this should be `bun build`

Now the system should be able call the blocks.  

All of these blocks should be installed in a fix location, `~/.spade/blocks`
