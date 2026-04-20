# Instructions

Please read the system specification in `../spec` and the skill for using this system in `../skills/spade`.  This will contain the necessary information for completing this part of the project.  

What we would like to do is create a large collection of pipelines in the `./pipelines` folder, and one or more scripts to run them using the spade CLI.  This will make a comprehensive integration test for the system.  For this reason, please create pipelines (YAML files) that cover the entire block collections that have been implemented so far in `../blocks/`, specifically the base, gdal, and data collections.  There should be in a suitable condition for testing end-to-end in the blocks.  

As an implementation note, the pipelines currently require generating IDs for the blocks.  This may mean that using a script to generate them is a better approach.  If using Python for this, please use the `uv` package manager for consistency with the rest of the system, or if using TypeScript, please use `Bun`.  You can of course use bash as well. One of these should be fine for the task if needed.  

Please create one or more scripts to run the tests, writing the output to `./output`.  Please also print the number of errors to stdout when running the script.  There is also a folder for logs, `./logs`.  