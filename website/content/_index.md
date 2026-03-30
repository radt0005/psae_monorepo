+++
title = "Spade"
template = "index.html"

[extra]
tagline = "Data processing at scale. Built for geospatial."
subtitle = "Build powerful data pipelines with reusable blocks in any language."
+++

```python
from spade import run, RasterFile

def handler(source: RasterFile) -> RasterFile:
    # Your processing logic here
    return RasterFile(path=result)

if __name__ == "__main__":
    run(handler)
```

```r
library(spade)

handler <- function(source) {
  # Your processing logic here
  RasterFile(path = result)
}

run(handler)
```

```typescript
import { run, RasterFile } from "spade";

function handler(source: RasterFile): RasterFile {
  // Your processing logic here
  return new RasterFile(resultPath);
}

run(handler);
```

```go
package main

import "github.com/spade-dev/spade-go"

func handler(source spade.RasterFile) spade.RasterFile {
    // Your processing logic here
    return spade.NewRasterFile(resultPath)
}

func main() {
    spade.Run(handler)
}
```

```rust
use spade::{run, RasterFile};

fn handler(source: RasterFile) -> RasterFile {
    // Your processing logic here
    RasterFile::new(result_path)
}

fn main() {
    run(handler);
}
```
