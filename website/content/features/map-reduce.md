+++
title = "Map/Reduce at Scale"
weight = 4
description = "Fan out across collections and reduce results back together, automatically."
template = "features/page.html"
+++

Spade's map/reduce system lets you process collections of data in parallel, then combine the results.

## Map Blocks

A map block takes a collection as input and produces an **expansion manifest** — a list of items to process independently. The scheduler then creates one invocation per item, executing them in parallel across available workers.

Each invocation receives:
- Its specific item from the collection
- All non-mapped inputs (broadcast automatically)
- A map context with the item index and total count

## Reduce Blocks

After all map invocations complete, a reduce block collects the results into a single output. The reducer receives an ordered collection of outputs from the map phase.

## How It Works

1. **Map block** enumerates items in the input collection
2. **Scheduler** creates N parallel invocations (one per item)
3. **Workers** execute each invocation independently and in parallel
4. **Reduce block** combines all outputs into a single result

## Broadcasting

Non-mapped inputs to a map block are automatically **broadcast** to every invocation. This means shared reference data (like a region of interest or configuration) is available to every parallel worker without duplication in the pipeline definition.
