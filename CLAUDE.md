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
- Go: `src/scratch-dna.go` - Goroutine-based parallel implementation with progress tracking

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

## Building & Setup

### Prerequisites

**For all tools:**
- Bash shell
- Python 3.x (standard library only - no external dependencies)

**For Go programs:**
- Go 1.22 or later (for modern `math/rand/v2`)

**For database benchmarks:**
- `pgbench_looper.sh`: PostgreSQL (`psql`, `createdb`, `pgbench`)
- `sysbench-mysql.sh`: MySQL (`mysql`, `sysbench`)

**For system utilities (database scripts):**
- `lscpu`, `lspci`, `sysctl` (for system metadata capture)

**For R benchmark:**
- R runtime with `rbenchmark` package

### Go Programs

The Go implementation of scratch-dna can be built for multiple platforms:

```bash
# Single platform (current OS)
cd src
go build -ldflags="-s -w" -o ../bin/scratch-dna-go scratch-dna.go

# Cross-platform builds
cd src
./build-all.sh scratch-dna.go
```

This produces binaries for:
- Linux (amd64)
- macOS/Darwin (amd64)
- Windows (amd64)

Build options: `-ldflags="-s -w"` for stripped, smaller binaries

### Python Scripts

Make executable:
```bash
chmod +x scratch-dna.py filecopy-benchmark.py fs-crawler.py
```

No build step required - runs directly with Python 3.

### C Programs

Build scratch-dna.c:
```bash
gcc -O3 -o scratch-dna scratch-dna.c -lm
```

For parallel operation, use the `-p` flag (requires fork support)

## Architecture Notes

### Multi-language DNA Writers

Three implementations (Python, C, Go) provide the same core functionality but with different parallelism models:
- **Python** (`scratch-dna.py`): Simple, single-threaded, good for baseline measurements
- **C** (`scratch-dna.c`): Fork-based multiprocessing using traditional Unix model
- **Go** (`bin/scratch-dna`): Goroutines with channels for progress reporting, most feature-rich

All implementations ensure:
- Consistent random data generation (single DNA string per run)
- Hostname-based output directories to support multi-node testing
- File naming: `scr-{hostname}-file-{counter}-{multiplier}` or similar patterns

**When to use each:**
- **Python**: Quick prototyping, baseline single-threaded performance
- **C**: Fork-based parallelism on Unix systems, minimal dependencies
- **Go**: Production benchmarks requiring parallel I/O, detailed progress tracking, robust error handling

### Output Directory Structure

All scratch-dna implementations create subdirectories by hostname to avoid clashes:
```
/output-dir/
├── hostname/
│   ├── scr-hostname-file-1-1
│   ├── scr-hostname-file-2-3
│   └── ...
```

Go version adds random suffix to hostname to handle parallel writers on same machine:
```
/output-dir/
├── hostname-a3f2b1/
└── hostname-c7e9d0/
```

### Parallel Filesystem Walking

`fs-crawler.py` implements custom parallel directory traversal optimized for network filesystems:
- Uses 36 worker threads by default
- LIFO (Last In First Out) scheduling for better locality
- Can fall back to standard `os.walk()` with `--no-parallel` flag
- Skips `.snapshot` directories automatically
- Useful for finding bottlenecks in metadata performance on network shares

### Benchmarking Philosophy

The tools follow HPC benchmarking best practices:
- Multiple iterations for statistical validity
- System metadata capture (CPU, RAM, kernel params)
- Support for distributed testing (hostname-based namespacing)
- Raw throughput AND files-per-second metrics (critical for HPC workloads)
- Results exportable to monitoring systems (Prometheus format support)

### Platform-Specific Considerations

**Linux/Unix:**
- All tools work natively
- C programs support fork-based parallelism
- Shell scripts require bash (test with `#!/bin/bash`)
- Direct I/O support for sysbench

**macOS:**
- Go binaries available for Darwin/amd64
- Python scripts work with native Python 3
- C compilation works with clang/gcc
- Some tools may require Homebrew packages (pgbench, sysbench)

**Windows:**
- Go binaries available for Windows/amd64
- Python scripts work with Windows Python 3.x
- C compilation requires MinGW or similar
- Database scripts may require WSL or adaptation
- `filecopy-benchmark.py` supports xcopy

### Error Handling & Debugging

**Go version specific errors:**
```bash
# Directory doesn't exist
2025/10/18 15:45:14 scratch-dna.go:57: Directory error: stat /nonexistent: no such file or directory

# Write permission denied
scratch-dna.go:102: Worker 3 write error: open /readonly/file: permission denied
```

All errors include:
- Source file and line number
- Worker ID (for parallel operations)
- Operation type (stat, open, write, etc.)
- Full file path for context

**Database script issues:**
- Ensure databases are installed: `psql --version`, `mysql --version`
- Run with sudo for system metric capture: `lscpu`, `lspci`, `sysctl`
- Check disk space - large tests require significant storage

## Running Benchmarks

### Storage Benchmarks

**Metadata performance test (small files):**
```bash
# Test 10,000 tiny files (4 bytes each)
./scratch-dna.py 10000 4 1 /tmp/test
# Output: Files written, throughput (MB/s), files/second (FPS)
```

**Throughput test (large files):**
```bash
# Go version: 1000 files, 1MB each, up to 10x duplication, 8 workers, verbose
./bin/scratch-dna -v -p 8 1000 1048576 10 /tmp/benchmark

# Python version: single-threaded baseline
./scratch-dna.py 1000 1048576 10 /tmp/benchmark

# C version: fork-based parallelism
./scratch-dna -p 8 1000 1048576 10 /tmp/benchmark
```

**Expected output:**
- Files written count
- Total size written (human-readable: GiB/MB)
- Elapsed time (seconds)
- Average FPS (files/second)
- Average throughput (MiB/s)

### Filesystem Analysis

**Quick filesystem crawl (without stats):**
```bash
./fs-crawler.py -f /path/to/scan -n
```

**Find recently modified files (last 30 days):**
```bash
./fs-crawler.py -f /path/to/scan -d 30
```

**Compare two directory trees for sync:**
```bash
./fs-crawler.py -f /source -t /target
```

### Network Share Performance

Test file copy performance to SMB/NFS shares:
```bash
./filecopy-benchmark.py
```

Tests 4 profiles with files ranging from 50KB to 300MB. Can output to Prometheus format.

### Database Performance Testing

**PostgreSQL (requires superuser):**
```bash
sudo ./pgbench_looper.sh
# Tests read-only, read-write, and custom workloads
# Outputs: detailed logs and CSV format
```

**MySQL (requires superuser):**
```bash
sudo ./sysbench-mysql.sh
# Tests OLTP workload with exponential thread scaling
# 1, 2, 4, 8, 16, 32, 64, 128 threads
```

### Compute Benchmarking

**R statistical computing (requires R + rbenchmark):**
```bash
./rbenchmark.R
# Tests array allocation and replication performance
```

## Common Development Workflows

### Quick Benchmark Test
```bash
# Go version (fastest, most features)
./bin/scratch-dna -v -p 4 100 1048576 5 /tmp/bench

# Or Python for minimal setup
./scratch-dna.py 100 1048576 5 /tmp/bench
```

### Modify and Test Go Code
```bash
# Make changes to src/scratch-dna.go
vim src/scratch-dna.go

# Rebuild
cd src
go build -ldflags="-s -w" -o ../bin/scratch-dna-go scratch-dna.go

# Test
../bin/scratch-dna-go -v -p 2 50 1024 2 /tmp/test
```

### Add a New Benchmark Tool
1. Follow existing patterns (use `scratch-dna.go` as reference for Go)
2. Include usage documentation in comments
3. Support verbose output for progress tracking
4. Capture and report key metrics (throughput, FPS, timing)
5. Handle errors gracefully with context

### Testing Output Quality
Check that benchmark output includes:
- Progress reporting (if applicable)
- Final metrics summary (throughput, FPS, elapsed time)
- Error messages with full context (file path, line number, worker ID)
- Human-readable file sizes (GiB, MiB, KB)

## Dependencies

### Python Scripts
- Python 3.x
- Standard library modules only (no external dependencies)
- Note: `requests` import was removed in recent commits

### Go Programs
- Go 1.22 or later (for `math/rand/v2`)
- No external Go modules required (standard library only)
- Cross-platform: Linux, macOS, Windows support

### Shell Scripts
- bash
- Database tools: `psql`, `createdb`, `mysql` (depending on test)
- System benchmarking: `sysbench`, `pgbench`
- System utilities: `lscpu`, `lspci`, `sysctl`

### R Scripts
- R runtime
- `rbenchmark` package

## Git Workflow

Main branch: `master` (use for pull requests)
Development branch: `dev` (current active branch)

**Before committing:**
- Test your changes: run the relevant benchmark tool
- Verify cross-platform compatibility for Go changes (test with `./src/build-all.sh`)
- Check that error messages are clear and include file paths
- Update CLAUDE.md if adding new tools or commands
