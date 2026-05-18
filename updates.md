# Project Update: May 2026

It has been a while since our last update (late July 2025), and a lot has changed under the hood. This note catches you up on where the project stands, what is coming next, and what we will be asking of you in June and July.

Before we get to that, though, there is an important update. The system now has a name: **Spade**.

## The short version

- Over the last nine months we have rebuilt the system from the prototype into something we believe can actually carry our analyses at scale. There is no new graphical interface to show off yet — most of the work was in the engine
  underneath — but the new engine is dramatically faster, more secure, and ready for real use.
- Dr. Phil Radtke's Fay-Herriot small-area estimator, which has been running on the old prototype, is being moved onto Spade. We are targeting **mid-June 2026** to have draft SAE blocks in place so we can demonstrate the full estimator on the new system.
- We will hold a **Zoom feedback session in mid-June 2026**, roughly one month before our **demo in July 2026 in Athens, GA**. The goal of the Zoom is to get your reactions while there is still time to act on them before the demo.

## Rationale

While the system prototype was working at the last PSAE meeting, there are a few things that we wanted to focus on to make the system work with the new NCASI agreement.  This follow up agreement includes CONUS wide execution of SAE workflows, and work with Dr. Radtke to move SAE methods onto the system.  While working on this, some things became clear: 

1. The prototype code needed to be cleaned, unified, and refactored to maintain velocity

2. The way work is executed needs to be updated to work with CONUS scales

3. The way blocks pass data needs to be updated

To this end, we have done a lot of backend work to overhaul the internals, move fractured code into one repository and one language (Go).  These internal changes support the long term health of the project, simplifying everything from the installation of the development tooling to further development to hosting on the cloud. 

## What changed since July 2025

When you last saw the system, it was a working prototype: useful for proving the ideas, but not something we could open up to outside users or run real production work on. The work since then has gone into closing that gap.

### A much faster work scheduler

The biggest change is in how the system distributes work. In the prototype, a pipeline ran on a single machine from start to finish. On the new system, each step of a pipeline can run on a different machine, and `map`/`reduce` operations (think: "for each tile, do X, then combine") fan out across the full pool of workers in parallel. For workflows like Phil's — where the same operation runs over hundreds of tiles or counties — this is the difference between hours and minutes.

The scheduler is also now **multi-tenant**: many users can submit pipelines at the same time, and the system will share workers fairly between them instead of one user's job blocking everything.  This work is happening now; the core algorithms have been implemented and we are validating this part of the system now. 

### A security model we can actually host

Every block now runs inside a sandbox (Ubuntu's `isolate`) that strictly limits what it can read, write, and reach over the network. Network access is off by default and must be declared explicitly by the block author. This is what makes it safe to let outside collaborators run code on a shared instance. 

### Language support

The prototype supported Python and R. Spade now also supports **Go**, **Rust**, and **TypeScript** (via Bun) for people writing their own blocks. You do not need to care about this directly — the choice of language is up to whoever writes a given block. What you do benefit from is the result: the core blocks have been rewritten in Rust, which runs noticeably faster and uses far less memory than the prototype's Python equivalents.  

### A library of building blocks

We have built out three first-party block collections so that you do not have to start from scratch to assemble a pipeline:

- **`blocks/base`** — common tabular operations: filter rows, select columns, group/aggregate, join, stack, convert between CSV and Parquet, plus the `map`/`reduce` primitives that drive parallel execution. Written in Rust.
- **`blocks/gdal`** — wrappers around the GDAL toolkit: rasterize, reproject, warp, clip, mosaic, contour, slope/aspect/hillshade, tile, and the rest of the usual geoprocessing operations. 
- **`blocks/data`** — connectors to common public data sources: Census ACS, Census TIGER, FIA, NHD, NLCD, OpenStreetMap, PRISM, SSURGO, USGS 3DEP, Natural Earth. A pipeline can pull data straight from these sources without you needing to download files by hand.

The SAE collection (`blocks/sae`) is being scaffolded now and is the near-term focus. More on this below.

### A new command-line tool

There is a new command-line tool (`spade`) that block authors and pipeline developers will use locally. This is mostly developer-facing and will not be the main way you interact with the system once the web interface is live. But it is worth knowing it exists, because the development team are using it, and because it is how the blocks are being authored and tested. 

### Testing improvements

We now have **83 integration-test pipelines** running on every change, in addition to a much tighter set of unit tests. Each integration test runs an end-to-end pipeline on real data and verifies the outputs. The goal is that by the time you are running your own analyses, the building blocks underneath them have been exercised on dozens of realistic workflows already.

### Better metadata for the graphical editor

When we eventually do show you the graphical pipeline editor, you should not have to read documentation to figure out what a block does or how its inputs connect. Block metadata has been expanded to include descriptions, expected formats, and human-readable labels for inputs and outputs, all of which the editor reads when you wire blocks together.

### Draft websites and documentation

We have stood up draft versions of a project website and a documentation site. They are not public yet, but they will be the landing places for new users and contributors once everything goes live.

## SAE on Spade

Dr. Radtke's Fay-Herriot small-area estimator currently runs on the prototype system. The plan is to bring it across to Spade and have a working end-to-end SAE pipeline running on the new platform **by the mid-June Zoom session**. The goal is not feature-completeness but rather to have something concrete to show in the meeting, so that the discussion can be about what is right, what is wrong, and what is missing, rather than abstract.

## A note on what you cannot do yet

The honest current state: Spade is **developer-only** today. There is no public sign-up, no hosted web editor pointed at you, and we do not yet expect you to log in and try things. The hosted instance and an invitation list are part of what we are building toward in the meantime. We will have another update related to that later on. 

Why is this?  There is a simple reason: the user interface is only 4% of the system code.  These updates are happening on the other 96% is where these updates are happening.  The current state of the system is that the user interface is not yet updated to work with the new overhauled backend of the system, but those will be relatively small changes compared to the other changes. 

## Questions, ideas, datasets to flag

Reply to whoever sent you this note, or reach out directly. The earlier we hear from you, the more of it we can fold into the June session and the July demo.
