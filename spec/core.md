# Spade


## Introduction 

The Spade system is a data processing system for massive data.  The system is plugin-based and has first-class support for geospatial data. Each processing step is a separate "block" executed independently.  This means that the system is extensible and scalable, since blocks can be executed in parallel and additional blocks can be developed later. 

The system is based on the concept of "pipelines" and "blocks".  Each data processing workflow is written as a pipeline: a sequence of steps that can be executed to get the desired results. Each of these steps is carried out by a block.  These are executed by the scheduler either in sequence or in parallel depending on the requirements, and the relationships between the blocks (for example, one block needing another block's output as input; this enforces an execution order).  

## Architecture

The system has six components:
1. The scheduler.  This system schedules the execution of blocks on worker nodes.
2. The worker nodes.  These are independent worker nodes that do the actual block execution.  Workers do not share a filesystem -- each worker uses its own local disk for scratch and a worker-local input cache, and data flowing between blocks moves through object storage (see `worker.md`)
3. The client. This is a web-based GUI where users submit and create jobs using a flowchart-like interface.  
4. The server layer.  Currently, we are using PocketBase here.  This component holds authentication and data for the client app and submit jobs for the scheduler. 
5. The CLI.  This command line application is the development tooling for the system and allows for running pipelines locally.  This includes a lot of tools for developing blocks as well.  
6. The blocks: these are the blocks that do the actual work.  These are effectively plugins, but the system won't work without them.  

### Blocks

There is a separate document on how the blocks work, but there is a brief summary here.

Each block executes as its own independent subprocess and works in its own directory.  It loads its arguments and everything it needs from the file system using the provided libraries. 

We provide three core libraries of blocks, and people can add their own
1. The core: these are common data operations, and we write them in rust for speed.
2. GDAL.  These blocks are wrappers on the Geospatial Data Analysis Library (GDAL).  These blocks are broadly responsible for enabling geospatial operations on the data. 
3. Data Providers: These blocks wrap the OpenDAL library (again, in Rust) and enable connecting to many different data sources with ease.  It also provides a library of known data sources that can be downloaded using the library in addition to the user-provided sources.

