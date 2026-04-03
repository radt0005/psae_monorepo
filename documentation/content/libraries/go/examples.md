+++
title = "Examples"
description = "Complete worked examples of Go blocks."
weight = 5
+++

These examples show complete, working Go blocks covering common patterns. Each example includes the block manifest, the handler implementation, and the directory layout at runtime.

## Example 1: Raster reprojection

A block that reads a raster file and a resolution parameter, reprojects the raster, and writes the result.

### Manifest (`blocks/reproject.yaml`)

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

### Handler (`reproject.go`)

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
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("gdalwarp failed: %s: %w", string(out), err)
	}

	result := spade.NewRasterFile(outputPath)
	return &result, nil
}
```

### Runtime directory layout

```
inputs/
  source/
    original.tif
params.yaml          # resolution: 10
outputs/
  raster/
    reprojected.tif
```

---

## Example 2: CSV data analysis

A block that reads a CSV file, computes summary statistics for a specified column, and writes the results as JSON.

### Manifest (`blocks/summarize.yaml`)

```yaml
id: data-tools.summarize
version: 0.1.0
kind: standard
network: false
description: Computes summary statistics for a CSV column

inputs:
  data:
    type: file
    format: CSV
  column:
    type: string

outputs:
  stats:
    type: json
```

### Handler (`summarize.go`)

```go
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"

	spade "github.com/spade-dev/spade"
)

func main() {
	spade.Run(summarize)
}

func summarize(args *spade.Args) (*spade.JsonFile, error) {
	data, err := spade.Input[*spade.TabularFile](args, "data")
	if err != nil {
		return nil, err
	}

	column, err := spade.Param[string](args, "column")
	if err != nil {
		return nil, err
	}

	// Read CSV
	f, err := os.Open(data.Path)
	if err != nil {
		return nil, fmt.Errorf("opening CSV: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV has no data rows")
	}

	// Find column index
	headers := records[0]
	colIdx := -1
	for i, h := range headers {
		if h == column {
			colIdx = i
			break
		}
	}
	if colIdx == -1 {
		return nil, fmt.Errorf("column '%s' not found", column)
	}

	// Compute statistics
	var values []float64
	for _, row := range records[1:] {
		if colIdx < len(row) {
			if v, err := strconv.ParseFloat(row[colIdx], 64); err == nil {
				values = append(values, v)
			}
		}
	}

	sum := 0.0
	minVal := math.Inf(1)
	maxVal := math.Inf(-1)
	for _, v := range values {
		sum += v
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	stats := map[string]any{
		"column": column,
		"count":  len(values),
		"mean":   sum / float64(len(values)),
		"min":    minVal,
		"max":    maxVal,
	}

	outputPath := "summary.json"
	out, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(outputPath, out, 0644); err != nil {
		return nil, err
	}

	result := spade.NewJsonFile(outputPath)
	return &result, nil
}
```

### Runtime directory layout

```
inputs/
  data/
    measurements.csv
params.yaml          # column: temperature
outputs/
  stats/
    summary.json
```

---

## Example 3: Map block -- batch raster processing

A map block processes each element of a collection independently. The Spade runtime invokes the handler once per input item.

### Manifest (`blocks/normalize.yaml`)

```yaml
id: raster-tools.normalize
version: 0.1.0
kind: map
network: false
description: Normalizes a raster file to 0-1 range

inputs:
  raster:
    type: file
    format: GeoTIFF

outputs:
  raster:
    type: file
    format: GeoTIFF
```

### Handler (`normalize.go`)

```go
package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	spade "github.com/spade-dev/spade"
)

func main() {
	spade.Run(normalize)
}

func normalize(args *spade.Args) (*spade.RasterFile, error) {
	raster, err := spade.Input[*spade.RasterFile](args, "raster")
	if err != nil {
		return nil, err
	}

	inputName := strings.TrimSuffix(filepath.Base(raster.Path), filepath.Ext(raster.Path))
	outputPath := fmt.Sprintf("%s_normalized.tif", inputName)

	cmd := exec.Command("gdal_calc.py",
		"-A", raster.Path,
		fmt.Sprintf("--outfile=%s", outputPath),
		"--calc=(A - A.min()) / (A.max() - A.min())",
		"--type=Float32",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("gdal_calc failed: %s: %w", string(out), err)
	}

	result := spade.NewRasterFile(outputPath)
	return &result, nil
}
```

When used in a pipeline with a collection of rasters, the Spade runtime calls this handler once per raster file. Each invocation sees a single raster in `inputs/raster/`.

### Runtime directory layout (per invocation)

```
inputs/
  raster/
    tile_001.tif
outputs/
  raster/
    tile_001_normalized.tif
```

---

## Example 4: Multiple outputs

A handler that produces both a processed raster and a JSON statistics file using the `Outputs` collection.

### Handler (`analyze.go`)

```go
package main

import (
	"encoding/json"
	"os"

	spade "github.com/spade-dev/spade"
)

func main() {
	spade.Run(analyze)
}

func analyze(args *spade.Args) (*spade.Outputs, error) {
	source, err := spade.Input[*spade.RasterFile](args, "source")
	if err != nil {
		return nil, err
	}

	threshold, err := spade.Param[float64](args, "threshold")
	if err != nil {
		return nil, err
	}

	// Processing logic...
	_ = source
	_ = threshold

	rasterOut := "classified.tif"
	// ... write raster ...

	statsOut := "classification_stats.json"
	stats := map[string]any{"threshold": threshold, "classified_pixels": 42000}
	data, _ := json.MarshalIndent(stats, "", "  ")
	os.WriteFile(statsOut, data, 0644)

	rasterResult := spade.NewRasterFile(rasterOut)
	jsonResult := spade.NewJsonFile(statsOut)

	outputs := spade.NewOutputs()
	outputs.Add("raster", &rasterResult)
	outputs.Add("stats", &jsonResult)

	return outputs, nil
}
```

### Runtime directory layout

```
inputs/
  source/
    input.tif
params.yaml          # threshold: 0.5
outputs/
  raster/
    classified.tif
  stats/
    classification_stats.json
```

The `Outputs.Add(name, value)` calls determine the subdirectory names under `outputs/`.
