+++
title = "Quickstart"
description = "Create your first Go block step by step."
weight = 1
+++

This guide walks you through creating a Spade block in Go. By the end you will have a working block that reads a raster file, applies a resolution parameter, and writes the result.

## Prerequisites

- **Go 1.25** or later
- The **Spade CLI** installed ([Installation guide](/getting-started/installation/))

## Step 1: Create a block collection

```bash
mkdir raster-tools && cd raster-tools
spade init --language go
```

This scaffolds the project:

```
raster-tools/
  go.mod
  main.go
  blocks/
```

Add the Spade library:

```bash
go get github.com/spade-dev/spade
```

## Step 2: Add a block

```bash
spade add reproject
```

This creates:

1. **`blocks/reproject.yaml`** -- the block manifest
2. **`reproject.go`** -- the handler entrypoint

## Step 3: Define the manifest

Edit `blocks/reproject.yaml`:

```yaml
id: raster-tools.reproject
version: 0.1.0
kind: standard
network: false
description: Reprojects a raster to a target resolution

inputs:
  source:
    type: file
    format: GeoTIFF
  resolution:
    type: number

outputs:
  raster:
    type: file
    format: GeoTIFF
```

## Step 4: Write the handler

Edit `reproject.go`:

```go
package main

import (
	"fmt"
	"os/exec"

	spade "github.com/spade-dev/spade"
)

func main() {
	spade.Run(reproject)
}

func reproject(args *spade.Args) (*spade.RasterFile, error) {
	source, err := spade.Input[*spade.RasterFile](args, "source")
	if err != nil {
		return nil, err
	}

	resolution, err := spade.Param[float64](args, "resolution")
	if err != nil {
		return nil, err
	}

	outputPath := "reprojected.tif"
	cmd := exec.Command("gdalwarp",
		"-tr", fmt.Sprintf("%f", resolution), fmt.Sprintf("%f", resolution),
		source.Path, outputPath,
	)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gdalwarp failed: %w", err)
	}

	result := spade.NewRasterFile(outputPath)
	return &result, nil
}
```

Key concepts:

- **`spade.Run[O IntoOutput](handler)`** is the generic entry point. The type parameter `O` is the handler's return type. The library infers it from your handler's signature.
- **`spade.Input[T](args, name)`** retrieves a typed file input by name. `T` must be a pointer to a type implementing `FromInput`.
- **`spade.Param[T](args, name)`** retrieves a typed scalar parameter from `params.yaml`.
- The handler receives an `*Args` struct and returns `(O, error)`.

## Step 5: Validate and install

```bash
spade check
spade install file://.
```

## Step 6: Use in a pipeline

```yaml
blocks:
  - id: "@reproject"
    name: raster-tools.reproject
    inputs: []
    args:
      resolution: 10
```

## No-output handlers

If your block performs a side effect without producing output, use `RunNoOutput`:

```go
func main() {
	spade.RunNoOutput(validate)
}

func validate(args *spade.Args) error {
	source, err := spade.Input[*spade.RasterFile](args, "source")
	if err != nil {
		return err
	}
	fmt.Printf("Validated: %s\n", source.Path)
	return nil
}
```

## Next steps

- [Types](/libraries/go/types/) -- all available Spade types
- [Handler Functions](/libraries/go/handlers/) -- handler patterns and multiple outputs
- [Manifest Generation](/libraries/go/manifest-generation/) -- generating manifests with the builder API
- [Examples](/libraries/go/examples/) -- complete worked examples
