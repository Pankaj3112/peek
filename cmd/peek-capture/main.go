package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/aymanbagabas/go-pty"
)

func main() {
	outputPath := flag.String("o", "", "output file path for captured pty bytes")
	flag.Parse()

	// Check required args
	if *outputPath == "" {
		fmt.Fprintf(os.Stderr, "usage: peek-capture -o <path> -- <command> [args...]\n")
		os.Exit(1)
	}

	// Get args after --
	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "error: no command provided after --\n")
		os.Exit(1)
	}

	// Open output file
	outFile, err := os.Create(*outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to open output file: %v\n", err)
		os.Exit(1)
	}
	defer outFile.Close()

	// Print to stderr so user knows where file is going
	fmt.Fprintf(os.Stderr, "peek-capture: writing pty bytes to %s\n", *outputPath)

	// Create pty
	p, err := pty.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create pty: %v\n", err)
		os.Exit(1)
	}
	defer p.Close()

	// Set pty size to 80x24
	if err := p.Resize(80, 24); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to resize pty: %v\n", err)
		os.Exit(1)
	}

	// Create command attached to pty
	cmd := p.Command(args[0], args[1:]...)

	// Start stdin forwarding goroutine
	go func() {
		if _, err := io.Copy(p, os.Stdin); err != nil && err != io.EOF {
			fmt.Fprintf(os.Stderr, "error: stdin copy failed: %v\n", err)
		}
	}()

	// Start pty output forwarding goroutine (to both file and stdout)
	outputDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.MultiWriter(outFile, os.Stdout), p)
		outputDone <- err
	}()

	// Handle SIGINT
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)

	// Start the command
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to start command: %v\n", err)
		os.Exit(1)
	}

	// Wait for SIGINT or command exit
	exitCode := 0
	select {
	case <-sigChan:
		// Forward SIGINT to the child
		if cmd.Process != nil {
			cmd.Process.Signal(syscall.SIGINT)
		}
		// Wait for child to exit
		if err := cmd.Wait(); err != nil {
			if exiterr, ok := err.(*exec.ExitError); ok {
				exitCode = exiterr.ExitCode()
			}
		}
	case <-outputDone:
		// Pty closed, wait for command
		if err := cmd.Wait(); err != nil {
			if exiterr, ok := err.(*exec.ExitError); ok {
				exitCode = exiterr.ExitCode()
			}
		}
	}

	// Close output file to ensure flush
	outFile.Close()

	os.Exit(exitCode)
}
