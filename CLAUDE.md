# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This repository contains scientific computing benchmarks focused on HPC (High Performance Computing), storage I/O performance, and database performance testing. The benchmarks measure throughput, files-per-second (FPS), and comparative CPU performance across different platforms.

## Key Components

### Storage Benchmarking Tools

**scratch-dna (Python, C, Go implementations)**
- Purpose: Multi-language storage benchmark that writes DNA-like random data to test storage I/O performance
- Python: `scratch-dna.py` - Single-threaded baseline implementation
- C: `scratch-dna.c` - Fork-based parallel implementation using `-p` flag
- Go: `src/scratch-dna-go.go` - Goroutine-based parallel implementation with progress tracking

**Usage patterns:**
```bash
# Python version (single-threaded)
./scratch-dna.py <num-files> <file-size-bytes> <random-multiplier> <output-dir>

# Go version (with parallelism and verbose output)
./scratch-dna-go -v -p 8 1000 1048576 10 /tmp
```

**Key behaviors:**
- Generates a single random DNA sequence (AGTC characters) once per run for consistency
- Each file gets the DNA sequence duplicated n times (n = random(1, multiplier))
- Creates subdirectories named after hostname to avoid clashes in multi-node scenarios
- Go version adds random suffix to hostname to prevent conflicts with parallel writers
- Outputs: throughput (MB/s), files-per-second (FPS), total size written

**filecopy-benchmark.py**
- Purpose: Tests file copy performance to network shares (SMB/NFS) with multiple file size profiles
- Runs 4 different test profiles: varying file sizes from 50KB to 300MB
- Can output results to Prometheus node exporter format for monitoring
- Tests both write throughput and delete performance

**fs-crawler.py**
- Purpose: Multi-threaded filesystem crawler for analyzing file metadata at scale
- Uses custom parallel walk implementation (36 threads by default) for network filesystems
- Can compare two directory trees for sync operations
- Useful for finding recently modified files and analyzing storage patterns

### Database Benchmarks

**pgbench_looper.sh**
- PostgreSQL performance testing with pgbench
- Tests read-only, read-write, and custom SQL workloads
- Varies client counts from 8 to 256 in increments of 8
- Captures system information (CPU, RAM, kernel params, disk controllers)
- Outputs results to both detailed logs and CSV format

**sysbench-mysql.sh**
- MySQL OLTP performance testing using sysbench
- Tests with exponentially increasing thread counts (1, 2, 4, 8, 16, 32, 64, 128)
- Runs multiple iterations per thread count for statistical validity
- Creates database with 10M rows by default

**sysbench-fileio.sh**
- File I/O testing with sysbench (NOTE: marked as not working due to file open errors)
- Tests multiple I/O patterns: seqwr, seqrd, rndrd, rndwr, rndrw
- Uses direct I/O flags to bypass OS cache

### Compute Benchmarks

**rbenchmark.R**
- R statistical computing benchmark
- Measures array allocation and replication performance
- Used to compare local hardware vs cloud instances (AWS C4)
- Documents impact of Meltdown patches on CPU-intensive workloads

## Building

### Go Programs

The Go implementation of scratch-dna can be built for multiple platforms:

```bash
cd src
./build-all.sh scratch-dna-go.go
```

This produces binaries for:
- Linux (amd64)
- macOS/Darwin (amd64)
- Windows (amd64)

Build options in use: `-ldflags="-s -w"` for stripped, smaller binaries

## Architecture Notes

### Multi-language DNA Writers

Three implementations (Python, C, Go) provide the same core functionality but with different parallelism models:
- **Python**: Simple, single-threaded, good for baseline measurements
- **C**: Fork-based multiprocessing using traditional Unix model
- **Go**: Goroutines with channels for progress reporting, most feature-rich

All implementations ensure:
- Consistent random data generation (single DNA string per run)
- Hostname-based output directories to support multi-node testing
- File naming: `scr-{hostname}-file-{counter}-{multiplier}` or similar patterns

### Parallel Filesystem Walking

`fs-crawler.py` implements custom parallel directory traversal optimized for network filesystems:
- Uses 36 worker threads by default
- LIFO (Last In First Out) scheduling for better locality
- Can fall back to standard `os.walk()` with `--no-parallel` flag
- Skips `.snapshot` directories automatically

### Benchmarking Philosophy

The tools follow HPC benchmarking best practices:
- Multiple iterations for statistical validity
- System metadata capture (CPU, RAM, kernel params)
- Support for distributed testing (hostname-based namespacing)
- Raw throughput AND files-per-second metrics (critical for HPC workloads)
- Results exportable to monitoring systems (Prometheus format support)

## Common Development Tasks

### Running Storage Benchmarks

Test small file metadata performance:
```bash
./scratch-dna.py 10000 4 1 /path/to/test/dir
```

Test throughput with larger files (Go version with 8 parallel writers):
```bash
./scratch-dna-go -v -p 8 1000 1048576 10 /path/to/test/dir
```

### Analyzing Filesystem Changes

Crawl a directory tree without stat calls (fast):
```bash
./fs-crawler.py -f /path/to/scan -n
```

Find files modified in last 30 days:
```bash
./fs-crawler.py -f /path/to/scan -d 30
```

### Database Performance Testing

Run PostgreSQL benchmarks:
```bash
sudo ./pgbench_looper.sh
```

Run MySQL benchmarks:
```bash
sudo ./sysbench-mysql.sh
```

Both require appropriate database software to be installed first.

## Dependencies

### Python Scripts
- Python 3.x
- Standard library modules only (no external dependencies)
- Note: `requests` import was removed in recent commits

### Go Programs
- Go compiler (for building from source)
- No external Go modules required (standard library only)

### Shell Scripts
- bash
- Database tools: `psql`, `createdb`, `mysql` (depending on test)
- System benchmarking: `sysbench`, `pgbench`
- System utilities: `lscpu`, `lspci`, `sysctl`

## Git Workflow

Main branch: `master` (use for pull requests)
Development branch: `dev` (current active branch)
