package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	// Configure logging with more details
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)

	// Define and set default command parameter flags
	var pFlag = flag.Int("p", 1, "Optional: set number of concurrent file writers to use; defaults to 1")
	var vFlag = flag.Bool("v", false, "Optional: Turn on verbose output mode; it will print the progress every second")
	var timeoutFlag = flag.Int("timeout", 600, "Optional: timeout in seconds for the entire operation (default: 600)")
	var syncFlag = flag.Bool("sync", false, "Optional: call fsync() after each file write (slower but ensures data on disk)")

	flag.Parse()
	args := flag.Args()

	if len(args) != 4 {
		fmt.Fprintf(os.Stderr, "Error! 4 positional arguments required.\n")
		fmt.Fprintf(os.Stderr, "\nUsage: %s [-p <parallel threads> -v (verbose) -sync -timeout <seconds>] <number-of-files> <file-size-bytes> <random-file-size-multiplier> <writable-directory>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nExample: %s -v -p 8 1024 10485760 1 /tmp\n\n", os.Args[0])
		os.Exit(1)
	}

	nFlag, err := strconv.Atoi(args[0])
	if err != nil || nFlag < 1 {
		log.Fatalf("Invalid number-of-files: %s (must be positive integer)", args[0])
	}

	sFlag, err := strconv.Atoi(args[1])
	if err != nil || sFlag < 1 {
		log.Fatalf("Invalid file-size-bytes: %s (must be positive integer)", args[1])
	}

	mFlag, err := strconv.Atoi(args[2])
	if err != nil || mFlag < 1 {
		log.Fatalf("Invalid random-file-size-multiplier: %s (must be positive integer)", args[2])
	}

	dFlag := args[3]

	// Validate directory exists and is writable
	if info, err := os.Stat(dFlag); err != nil {
		log.Fatalf("Directory error: %v", err)
	} else if !info.IsDir() {
		log.Fatalf("Path is not a directory: %s", dFlag)
	}

	if *pFlag < 1 {
		log.Fatalf("Parallel threads must be at least 1, got: %d", *pFlag)
	}

	// Generate random DNA sequence
	size := sFlag
	out := genstring(size)

	// Generate unique hostname identifier
	hostname, err := os.Hostname()
	if err != nil {
		log.Printf("Warning: Could not get hostname: %v, using 'dna'", err)
		hostname = "dna"
	}
	// Faster integer formatting using strconv
	hostname = hostname + "_" + strconv.Itoa(rand.IntN(100000000))

	// Create output directory once upfront to avoid race conditions
	outDir := filepath.Join(dFlag, hostname)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory %s: %v", outDir, err)
	}

	// Create context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutFlag)*time.Second)
	defer cancel()

	// Setup channels and synchronization
	wg := new(sync.WaitGroup)
	sema := make(chan struct{}, *pFlag)
	progress := make(chan int64, 256)
	errors := make(chan error, nFlag)

	var nfiles, nbytes atomic.Int64
	start := time.Now().Unix()

	// Progress monitor goroutine
	go func() {
		wg.Wait()
		close(progress)
		close(errors)
	}()

	fmt.Printf("Writing %d files with filesizes between %.1f MB and %.1f MB...\n", nFlag, float64(size)/1048576, float64(size)/1048576*float64(mFlag))
	fmt.Printf("Using %d parallel workers\n\n", *pFlag)

	// Launch file writers
	for x := 1; x <= nFlag; x++ {
		wg.Add(1)
		go spraydna(ctx, x, wg, sema, out, outDir, progress, errors, mFlag, hostname, *syncFlag)
	}

	// Progress ticker
	var tick <-chan time.Time
	if *vFlag {
		ticker := time.NewTicker(1000 * time.Millisecond)
		defer ticker.Stop()
		tick = ticker.C
	}

	// Collect errors
	var errCount int
	done := make(chan struct{})
	go func() {
		for err := range errors {
			if err != nil {
				log.Printf("Error: %v", err)
				errCount++
			}
		}
		close(done)
	}()

	// Main event loop
loop:
	for {
		select {
		case <-ctx.Done():
			log.Printf("ERROR: Operation timed out after %d seconds", *timeoutFlag)
			log.Printf("Files completed: %d/%d", nfiles.Load(), nFlag)
			os.Exit(1)
		case size, ok := <-progress:
			if !ok {
				break loop // progress was closed
			}
			nfiles.Add(1)
			nbytes.Add(size)
		case <-tick:
			printProgress(nfiles.Load(), nbytes.Load(), start, int64(nFlag))
		}
	}

	// Wait for error collector to finish
	<-done

	// Final totals
	printDiskUsage(nfiles.Load(), nbytes.Load(), start)

	if errCount > 0 {
		log.Printf("WARNING: Completed with %d errors", errCount)
		os.Exit(1)
	}
}


func genstring(size int) []byte {
	fmt.Printf("Building random DNA sequence of %.1f MB...\n\n", float64(size)/1048576)
	dnachars := []byte("GATC")
	dna := make([]byte, size)
	for x := 0; x < size; x++ {
		dna[x] = dnachars[rand.IntN(len(dnachars))]
	}
	return dna
}

func spraydna(ctx context.Context, count int, wg *sync.WaitGroup, sema chan struct{}, out []byte, dir string, progress chan<- int64, errors chan<- error, mFlag int, hostname string, doSync bool) {
	defer wg.Done()

	// Acquire semaphore token
	select {
	case sema <- struct{}{}:
		defer func() { <-sema }() // release token
	case <-ctx.Done():
		errors <- fmt.Errorf("worker %d: context cancelled while waiting for semaphore", count)
		return
	}

	multiple := rand.IntN(mFlag) + 1
	// Faster filename construction with strconv
	filename := filepath.Join(dir, hostname+"-"+strconv.Itoa(count)+"-"+strconv.Itoa(multiple)+".txt")

	f, err := os.Create(filename)
	if err != nil {
		errors <- fmt.Errorf("worker %d: failed to create file %s: %w", count, filename, err)
		return
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			errors <- fmt.Errorf("worker %d: failed to close file %s: %w", count, filename, closeErr)
		}
	}()

	w := bufio.NewWriterSize(f, 256*1024) // 256KB buffer for better performance
	writtenBytes := 0

	for j := 0; j < multiple; j++ {
		n, err := w.Write(out)
		if err != nil {
			errors <- fmt.Errorf("worker %d: write failed on iteration %d/%d to %s: %w", count, j+1, multiple, filename, err)
			return
		}
		writtenBytes += n
	}

	if err := w.Flush(); err != nil {
		errors <- fmt.Errorf("worker %d: flush failed for %s: %w", count, filename, err)
		return
	}

	// Only sync if explicitly requested (much faster without it)
	if doSync {
		if err := f.Sync(); err != nil {
			errors <- fmt.Errorf("worker %d: sync failed for %s: %w", count, filename, err)
			return
		}
	}

	// Successfully completed - send progress
	select {
	case progress <- int64(writtenBytes):
	case <-ctx.Done():
		errors <- fmt.Errorf("worker %d: context cancelled while reporting progress", count)
	}

	// Send nil error to indicate success
	errors <- nil
}

func printProgress(nfiles, nbytes, start, total int64) {
	now := time.Now().Unix()
	elapsed := now - start
	if elapsed == 0 {
		elapsed = 1
	}
	fps := nfiles / elapsed
	tp := nbytes / elapsed
	remaining := total - nfiles
	pct := float64(nfiles) / float64(total) * 100
	fmt.Printf("Progress: %6.2f%% | Files: %7d/%7d | Data: %5.1f GiB | FPS: %5d | Throughput: %4d MiB/s | Remaining: %7d\n",
		pct, nfiles, total, float64(nbytes)/1073741824, fps, tp/1048576, remaining)
}

// Prints the final summary
func printDiskUsage(nfiles, nbytes int64, start int64) {
	stop := time.Now().Unix()
	elapsed := stop - start
	if elapsed == 0 {
		elapsed = 1
	}
	fps := nfiles / elapsed
	tp := nbytes / elapsed
	separator := strings.Repeat("=", 80)
	fmt.Printf("\n%s\n", separator)
	fmt.Printf("Benchmark Complete!\n")
	fmt.Printf("%s\n", separator)
	fmt.Printf("Files Written:    %d\n", nfiles)
	fmt.Printf("Total Size:       %.2f GiB (%.2f MB)\n", float64(nbytes)/1073741824, float64(nbytes)/1048576)
	fmt.Printf("Elapsed Time:     %d seconds\n", elapsed)
	fmt.Printf("Avg FPS:          %d files/second\n", fps)
	fmt.Printf("Avg Throughput:   %d MiB/s\n", tp/1048576)
	fmt.Printf("%s\n", separator)
}
