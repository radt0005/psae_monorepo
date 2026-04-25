# User Interface Specification

The user interface should be web-based.

The user interface for this tool should be easy to use and make creating workflows easy.  Specifically, we have identified the following components that should be included in the user interface.  The UI should enable users to: 
1. Create and run pipelines using a flowchart-based interface
2. view results
3. download result files
4. re-use pipelines
5. share pipelines
6. Browse blocks
7. Share results
8. Upload custom data (maybe with a helper tool)

The system should have user authentication and limit access to authorized users.

This should be clean and intuitive for users, whether they aim to create custom workflows or use pre-populated ones.

## Input/Output Wiring

When a user connects two blocks in the flowchart editor, the UI must resolve which outputs from the upstream block feed into which inputs on the downstream block.

- **Unambiguous connections**: If the types match unambiguously (e.g. one GeoTIFF output to one GeoTIFF input), the connection is made automatically with no user interaction needed.
- **Ambiguous connections**: If the upstream block has multiple outputs that could satisfy the same input type on the downstream block, the UI should **prompt the user to choose** which output maps to which input.  This produces explicit output references in the pipeline YAML (`block` + `output` form).  The user should not be able to save or submit a pipeline with unresolved ambiguous connections.
- **Block info**: When connecting blocks, the UI should display the `description` field from each input and output declaration to help users make the correct mapping.  For example, a prompt might show: *"Block 'raster.split' has two GeoTIFF outputs: **tiles** (Individual raster tiles) and **boundary** (Extent polygon rasterized to grid). Which should connect to input **source** (Raster to process)?"*

This ensures that all pipelines produced by the UI are valid and unambiguous without requiring users to understand the underlying resolution algorithm.

## Pipelines

There should be a way to store and share pipelines for future use.  This means that they will need to be stored and have access control for them.  This should store the pipeline YAML files, and the pipelines should be loaded into the editor.

It should be possible to browse past pipelines and re-load one in the editor for re-running or editing.  

## Browsing Results

There should be a way to browse results, view results that are small enough, and download results through the UI. This should include large files as well. 

## User Interface for Pipeline Generation

Some notable ommissions from the interface.  There should be a way to upload (potentially large) files and use the custom data in pipelines.  There should also be a way to share this data as well without re-uploading the data (especially since geospatial files can be large). 


## Look and Feel

The user interface should have the same look and feel as the website and documentation website. This should include the same color palette, icons, and overall feel.  

There should 

