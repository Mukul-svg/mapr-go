# MapReduce in Go

A distributed MapReduce implementation in Go, inspired by the [original Google MapReduce paper](https://pdos.csail.mit.edu/6.824/papers/mapreduce.pdf).

## Architecture

```
┌─────────────────┐        Unix socket (RPC)        ┌──────────────┐
│   Coordinator   │ ◄─────────────────────────────► │   Worker 1   │
│                 │                                  └──────────────┘
│  - assigns map  │     ┌──────────────┐
│    & reduce     │ ◄──►│   Worker 2   │
│    tasks        │     └──────────────┘
│  - re-assigns   │     ┌──────────────┐
│    on timeout   │ ◄──►│   Worker N   │
└─────────────────┘     └──────────────┘
```

**Map Phase**
- One map task per input file
- Each worker reads its file, calls `Map()`, and hash-partitions output into `NReduce` buckets
- Intermediate files: `mr-<mapID>-<reduceID>`

**Reduce Phase**
- Starts after *all* map tasks finish
- Each worker reads its bucket across all `NMap` files, sorts by key, calls `Reduce()`
- Output files: `mr-out-<reduceID>` (merge them for the final result)

**Fault Tolerance**
- If a worker takes more than 10 seconds, the coordinator re-assigns its task
- Workers write to temp files and atomically rename (crash-safe)

## Project Structure

```
mapreduce-go/
├── mr/
│   ├── coordinator.go  # Coordinator: task assignment, fault tolerance
│   ├── worker.go       # Worker: map/reduce execution loop
│   └── rpc.go          # RPC message types
├── apps/
│   ├── wc.go           # Word count application
│   └── indexer.go      # Inverted index application
├── cmd/
│   ├── coordinator/    # Coordinator entry point
│   └── worker/         # Worker entry point
├── testdata/           # Sample input files (Project Gutenberg public domain)
└── scripts/
    └── test_wc.sh      # End-to-end word count test
```

## Requirements

- **Go 1.22+**
- **Linux or WSL** (uses Unix domain sockets)

## Build & Run

### 1. Build

```bash
go build -o coordinator ./cmd/coordinator
go build -o worker ./cmd/worker
```

### 2. Run Word Count

In one terminal, start the coordinator:

```bash
./coordinator /tmp/mr-sock testdata/pg-*.txt
```

In separate terminals, start workers (run at least 2–3):

```bash
./worker --app wc /tmp/mr-sock
./worker --app wc /tmp/mr-sock
./worker --app wc /tmp/mr-sock
```

Once the coordinator exits ("all tasks done"), merge the output:

```bash
sort mr-out-* > mr-wc-final.txt
head mr-wc-final.txt
```

### 3. Run the Automated Test

The test script builds, runs, and verifies the word count job:

```bash
bash scripts/test_wc.sh
```

Or with more workers:

```bash
bash scripts/test_wc.sh 5
```

### 4. Run Inverted Index

```bash
./coordinator /tmp/mr-sock testdata/pg-*.txt
./worker --app indexer /tmp/mr-sock   # (in multiple terminals)
sort mr-out-* > mr-indexer-final.txt
```

## Adding a New Application

1. Create a file in `apps/myapp.go`:

```go
package apps

import "mapreduce-go/mr"

func MyMap(filename string, contents string) []mr.KeyValue {
    // ... emit key/value pairs
}

func MyReduce(key string, values []string) string {
    // ... aggregate values for this key
}
```

2. Register it in `cmd/worker/main.go`:

```go
case "myapp":
    mapf = apps.MyMap
    reducef = apps.MyReduce
```

3. Run: `./worker --app myapp /tmp/mr-sock`

## How It Works

The coordinator and workers communicate entirely via RPC over a Unix socket.

| RPC | Direction | Description |
|---|---|---|
| `GetTask` | Worker → Coordinator | Request the next available task |
| `ReportTaskDone` | Worker → Coordinator | Signal that a task is complete |

Task lifecycle:
```
Idle → Inprogress → Completed
         ↑
    (timeout: reset to Idle after 10s)
```
