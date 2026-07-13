+++
title = "Handler Functions"
description = "Writing Go handlers with generics."
weight = 3
+++

A handler is a function that receives an `*Args` struct and returns a typed output along with an error. The Spade library calls your handler after loading all inputs and parameters.

## Basic handler pattern

Every handler follows this signature:

```go
func handler(args *spade.Args) (OutputType, error)
```

The `Run` function is generic over the output type:

```go
func main() {
    spade.Run(handler)
}

func handler(args *spade.Args) (*spade.RasterFile, error) {
    source, err := spade.Input[*spade.RasterFile](args, "source")
    if err != nil {
        return nil, err
    }

    resolution, err := spade.Param[float64](args, "resolution")
    if err != nil {
        return nil, err
    }

    // Process the raster...
    result := spade.NewRasterFile("result.tif")
    return &result, nil
}
```

## The `Run` function

```go
func Run[O IntoOutput](handler func(*Args) (O, error))
```

`Run` is the main entry point. It:

1. **Loads parameters** from `params.yaml`
2. **Scans inputs** from the `inputs/` directory
3. **Builds the `Args` struct** merging parameters and inputs
4. **Calls your handler**
5. **Writes outputs** to the `outputs/` directory
6. **Exits** with code 1 on any error, printing the message to stderr

The generic type parameter `O` is inferred from the handler's return type.

## Closures

A handler does not have to be a named function -- `Run` accepts any value matching the `func(*Args) (O, error)` signature, including an anonymous function literal. This is useful for capturing values (flags, config loaded in `main`) without threading them through `*Args`:

```go
func main() {
    threshold := parseThresholdFlag()

    spade.Run(func(args *spade.Args) (*spade.RasterFile, error) {
        source, err := spade.Input[*spade.RasterFile](args, "source")
        if err != nil {
            return nil, err
        }
        // threshold is captured from the enclosing scope
        result := spade.NewRasterFile("result.tif")
        return &result, nil
    })
}
```

Because `Run` infers its generic type parameter `O` from the closure's signature, inference occasionally fails (most often when returning `*spade.Outputs`); if it does, supply `O` explicitly: `spade.Run[*spade.RasterFile](func(args *spade.Args) (*spade.RasterFile, error) { ... })`.

## Accessing inputs

Use the `Input` generic function to retrieve typed file inputs:

```go
source, err := spade.Input[*spade.RasterFile](args, "source")
if err != nil {
    return nil, err
}
// source.Path is the file path on disk
```

The type parameter `T` must be a pointer to a struct implementing `FromInput`. The library scans the corresponding `inputs/<name>/` subdirectory and constructs the appropriate type:

- **Single file** in the subdirectory: calls `FromSingleFile(path)`
- **Multiple files** in the subdirectory: calls `FromMultipleFiles(paths)`

## Accessing parameters

Use the `Param` generic function to retrieve scalar parameters from `params.yaml`:

```go
resolution, err := spade.Param[float64](args, "resolution")
method, err := spade.Param[string](args, "method")
enabled, err := spade.Param[bool](args, "enabled")
```

The library handles YAML type conversions automatically. For example, a YAML integer `10` can be read as `float64`, `int`, or `int64`.

## Accessing secrets

Use `spade.GetSecret` to retrieve a credential the pipeline injected under a logical name, rather than reading it from `params.yaml` or the OS environment directly:

```go
dsn, err := spade.GetSecret("db")
if err != nil {
    return nil, err
}
// dsn is a connection string, e.g. "postgresql://user:pass@host:5432/db"
```

`GetSecret(name string) (string, error)` returns the secret value bound to the logical `name`, following the same `(value, error)` convention as `Input` and `Param`. The logical name is part of your block's contract, documented like any other parameter: the pipeline author binds it to one of their stored secrets via a `secrets:` map alongside `args:` in the pipeline YAML.

If `name` was not declared in the pipeline's `secrets:` map, or the bound secret failed to resolve, `GetSecret` returns `spade.ErrSecretNotFound`. A declared-but-unresolvable secret is a real error, not a silently empty string -- handle it the same way you would `ErrInputNotFound` or `ErrParamNotFound`.

`GetSecret` never talks to a keychain or key-management service itself. It only reads values the worker or CLI already injected into the process environment before your handler ran.

## Single output

When the handler returns a single typed value, the library writes it to `outputs/`. The output directory name is determined by:

1. The block manifest, if it declares exactly one output
2. Otherwise, the type's `DefaultOutputName()` (e.g., `"raster"` for `RasterFile`)

```go
func handler(args *spade.Args) (*spade.RasterFile, error) {
    // ...
    result := spade.NewRasterFile("result.tif")
    return &result, nil
}
// Written to outputs/raster/result.tif (or the manifest-declared name)
```

## Multiple outputs

Use the `Outputs` collection to return multiple named outputs:

```go
func handler(args *spade.Args) (*spade.Outputs, error) {
    source, err := spade.Input[*spade.RasterFile](args, "source")
    if err != nil {
        return nil, err
    }

    // Process and create output files...
    rasterResult := spade.NewRasterFile("processed.tif")
    jsonResult := spade.NewJsonFile("stats.json")

    outputs := spade.NewOutputs()
    outputs.Add("raster", &rasterResult)
    outputs.Add("stats", &jsonResult)

    return outputs, nil
}
```

This writes:

```
outputs/
  raster/
    processed.tif
  stats/
    stats.json
```

## No-output handlers

If your block performs a side effect (validation, logging) without producing files, use `RunNoOutput`:

```go
func main() {
    spade.RunNoOutput(validate)
}

func validate(args *spade.Args) error {
    source, err := spade.Input[*spade.RasterFile](args, "source")
    if err != nil {
        return err
    }
    fmt.Printf("File OK: %s\n", source.Path)
    return nil
}
```

`RunNoOutput` takes a handler with signature `func(*Args) error` and skips the output-writing step.

## Testing with `RunAt`

The `RunAt` function runs a handler against a specific base directory instead of the current working directory. This is useful for unit tests:

```go
func TestReproject(t *testing.T) {
    // Set up test directory with inputs/ and params.yaml
    base := t.TempDir()
    setupTestInputs(base)

    err := spade.RunAt(base, func(args *spade.Args) (*spade.RasterFile, error) {
        source, err := spade.Input[*spade.RasterFile](args, "source")
        if err != nil {
            return nil, err
        }
        result := spade.NewRasterFile(filepath.Join(base, "result.tif"))
        return &result, nil
    })
    if err != nil {
        t.Fatal(err)
    }

    // Verify outputs exist
    if _, err := os.Stat(filepath.Join(base, "outputs", "raster", "result.tif")); err != nil {
        t.Fatal("output file not found")
    }
}
```

Similarly, `RunNoOutputAt` exists for no-output handlers.

## Optional inputs and parameters

Use `HasInput` and `HasParam` on the `*Args` struct to check for optional arguments before accessing them:

```go
func handler(args *spade.Args) (*spade.RasterFile, error) {
    source, err := spade.Input[*spade.RasterFile](args, "source")
    if err != nil {
        return nil, err
    }

    // Optional mask input
    var mask *spade.VectorFile
    if args.HasInput("mask") {
        m, err := spade.Input[*spade.VectorFile](args, "mask")
        if err != nil {
            return nil, err
        }
        mask = m
    }

    // Optional buffer parameter with default
    buffer := 0.0
    if args.HasParam("buffer") {
        b, err := spade.Param[float64](args, "buffer")
        if err != nil {
            return nil, err
        }
        buffer = b
    }

    // Use source, mask (may be nil), and buffer...
    result := spade.NewRasterFile("result.tif")
    return &result, nil
}
```

## Error handling

Return errors from your handler using standard Go error wrapping:

```go
func handler(args *spade.Args) (*spade.RasterFile, error) {
    source, err := spade.Input[*spade.RasterFile](args, "source")
    if err != nil {
        return nil, err
    }

    if !strings.HasSuffix(source.Path, ".tif") {
        return nil, fmt.Errorf("expected GeoTIFF, got %s", filepath.Ext(source.Path))
    }

    // ...
}
```

`Run` catches errors, prints them to stderr with a `spade:` prefix, and exits with code 1.
