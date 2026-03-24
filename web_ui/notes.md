# Development Notes

## TODO before March 27

### Overall

- [x] Define the Directory Structure and Controls
  - [x] Implement the directory structure
  - [x] Create the Directory Structure

### For the Client UI

- [x] Update the block list to pull from the API
- [x] Update the editor to pull Schema from the API
- [ ] Create Endpoints:
  - [ ] Run a pipeline [stubbed]
  - [x] List the results from the folder of output
  - [x] Fetch the results from a text-encoded file (CSV, JSON, GeoJSON)
  - [ ] Download results component (later)
- [x] Create the UI components to display a file (one by March)
  - [x] CSV as a NuxtUI Table
  - [x] GeoJSON as Leaflet map
  - [x] JSON as formatted text
  - [x] Download button
- [x] Investigate Async Updates (e.g. Server-sent Events)
- [x] Define the layout of the results, folders, etc.
- [x] Update the Block editor to remember the selected block by ID

### Runtime Connector

- [ ] runtime connector updates
  - [x] Update the R interface
  - [ ] update to include metadata about the pipeline
    - [ ] working directory
    - [ ] pipeline ID
    - [ ] Block ID
    - [ ] How to parse things into a filepath
  - [x] Create a to_path function that converts the string path OR uuid to a valid Path !!
  
### Integration
  - [ ] Connect Web App and Worker Nodes via RabbitMQ
    - [x] Worker Node
    - [ ] Nuxt App (WIP)

### CLI Runner

- [ ] CLI updates
  - [ ] Create Installer (later)
  - [ ] Create system initializer -> CLI (later)

### Testing

- [ ] Run end-to-end tests
- [x] Create Demo Script

## Directory Structure

```
~/.psae/
├── blocks/
│   ├── sql-duckdb-query/
│   │    ├── manifest.json
│   │    └── schema.json
│   ├── random-forest-fit/
│   │    ├── manifest.json
│   │    └── schema.json
│   ├── random-forest-predict/
│   │    ├── manifest.json
│   │    └── schema.json
├── data
│   └── # cached data files for re-use
├── config.json (config file)
├── runs/
│   ├── dc9c06d3-8123-425a-b729-2a756623fe46/
│   │    └── # files for the run
│   └── e76b11ce-5fb5-400e-a3fb-9b3da2d863bd/
│   │    └── # # files for the run
└── utils/
│   └── # Might need this for block templates and other assets?
```

One important consideration here is the way R blocks are handled.  It should be possible to bundle the R code with teh Python code, and use the Python runtime to act as a bridge.

I think that installing blocks with pipx is a great way to handle the Python Blocks.  This would work as follows:

### Python + Pipx

This system gives a lot of flexibility in the way that the blocks are Handled.  

The user can then use any build system that creates a wheel.  Then we can install the wheel with pipx, and then use

```bash
pipx run <blockname> [args]
```

to run it.  The installer will run the package build command, and then the pipx install command.  For example, with uv this would look like:

```bash
uv build
pipx install .
```

This should work just fine.  It's also possible to use uv for everything, or use another system runner, but I think that this is the best way.  

#### R Blocks with pipx

It might be possible to include the R blocks as well with pipx.  This would mean packaging an R block with the Python runtime and a connector.  This is probably possible, just a matter of getting the build correct.  

There are Python-R connectors, so we might be able to use those to make all blocks Python blocks, and the python blocks run R code.

It should be possible to use `rpy2` as a bridge from the python runtime to the user's R handlers.  Then all we need to do is the following: 

1. In an R block template, inlcude a build.py file
2. The block manifest.json should include the fact that it's using R, the name of the entrypoint script, and the name of the handler. 
3. The build script should load the R bridge R code from a file.  This code should use the `compileR` package to compile file specified by the user into `main.Rc`.  It should also convert the docstrings into a JSON schema.
4. The compiled file should be included as an asset in the pyproject.toml
5. The python block can be built, and the main.Rc file should be included in the project wheel

This requires that the `main.Rc` file (and other dependencies, e.g. block manifest) are listed as inclusions in the pyproject.toml file: 

```toml
[tool.setuptools.package-data]
myModule = ["*.Rc"]
```

Note this requires setuptools versions over 60.0 as this is a relatively new feature.

Now for running the block:

1. The Python Runtime is invoked.  Load the Manifest
2. Call the R runtime bridge.  This loads the compiled block from the bundled `main.Rc` file using R.
3. The handler is invoked using rpy2. 
4. The output is returned to Python
5. Python runtime saves the data to a file (unless the R instance does, which might be easier)
