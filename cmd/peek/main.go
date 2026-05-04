package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/Pankaj3112/peek/internal/cli"
	"github.com/Pankaj3112/peek/internal/mcp"
	"github.com/Pankaj3112/peek/internal/wrapper"
	"golang.org/x/term"
)

const version = "dev"

func main() {
	args := os.Args[1:]

	// Check for -- separator for wrap mode
	dashIdx := -1
	for i, arg := range args {
		if arg == "--" {
			dashIdx = i
			break
		}
	}

	// If -- found, split into peek args and wrapped command
	if dashIdx >= 0 {
		peekArgs := args[:dashIdx]
		wrappedArgs := args[dashIdx+1:]

		// Parse peek's own flags before --
		fs := flag.NewFlagSet("peek", flag.ContinueOnError)
		fs.SetOutput(io.Discard) // Suppress default error output
		version := fs.Bool("version", false, "")
		help := fs.Bool("help", false, "")
		h := fs.Bool("h", false, "")

		if err := fs.Parse(peekArgs); err != nil {
			fmt.Fprintf(os.Stderr, "peek: %v\n", err)
			os.Exit(2)
		}

		if *version {
			fmt.Println("peek dev")
			os.Exit(0)
		}
		if *help || *h {
			printHelp(os.Stdout)
			os.Exit(0)
		}

		// If there are any unparsed args (not flags), it's an error
		if fs.NArg() > 0 {
			printUnknownSubcommand(fs.Args()[0])
			os.Exit(2)
		}

		// Wrap mode: must have at least one arg after --
		if len(wrappedArgs) == 0 {
			fmt.Fprintf(os.Stderr, "wrap: no command provided\n")
			os.Exit(1)
		}

		handleWrap(wrappedArgs)
		return
	}

	// No -- found; parse normal subcommands
	if len(args) == 0 {
		// No args at all: print help and exit 2
		printHelp(os.Stdout)
		os.Exit(2)
	}

	// Check for global flags
	if args[0] == "--version" {
		fmt.Println("peek dev")
		os.Exit(0)
	}
	if args[0] == "--help" || args[0] == "-h" {
		printHelp(os.Stdout)
		os.Exit(0)
	}

	// Dispatch subcommands
	switch args[0] {
	case "mcp":
		handleMCP(args[1:])
	case "list":
		handleList(args[1:])
	case "logs":
		handleLogs(args[1:])
	default:
		printUnknownSubcommand(args[0])
		os.Exit(2)
	}
}

func handleMCP(_ []string) {
	binary, _ := os.Executable()
	srv := mcp.NewServer(os.Stdin, os.Stdout, os.Stderr, version, binary)
	mcp.RegisterListSessions(srv)
	mcp.RegisterGetLogs(srv)
	// search_logs handler will be registered in Task 38.
	if err := srv.ServeUntilEOF(); err != nil {
		fmt.Fprintf(os.Stderr, "peek mcp: %v\n", err)
		os.Exit(1)
	}
}

func handleList(_ []string) {
	width := getTerminalWidth(os.Stdout)
	if err := cli.RenderList(os.Stdout, width); err != nil {
		fmt.Fprintf(os.Stderr, "peek list: %v\n", err)
		os.Exit(1)
	}
}

func getTerminalWidth(f *os.File) int {
	fd := int(f.Fd())
	if !term.IsTerminal(fd) {
		return 100
	}
	w, _, err := term.GetSize(fd)
	if err != nil {
		return 100
	}
	return w
}

func handleLogs(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "peek logs: missing id argument")
		os.Exit(2)
	}
	id, err := cli.ResolveIDPrefix(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "peek logs: %v\n", err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := cli.RenderLogs(ctx, os.Stdout, id); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "peek logs: %v\n", err)
		os.Exit(1)
	}
}

func handleWrap(args []string) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "peek: failed to get working directory: %v\n", err)
		os.Exit(1)
	}
	cwd = filepath.Clean(cwd)

	exitCode, err := wrapper.Wrap(cwd, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "peek: wrap failed: %v\n", err)
		os.Exit(1)
	}
	os.Exit(exitCode)
}

func printHelp(w io.Writer) {
	help := `peek - CLI wrapper for dev server logs

Usage:
  peek [global flags] <subcommand> [args...]
  peek -- <cmd> [args...]

Global flags:
  --version  Print version and exit
  --help, -h Print this help and exit

Subcommands:
  mcp        Start MCP server
  list       List captures
  logs <id>  Show logs for a capture

Wrap mode:
  peek -- <cmd> [args...]  Wrap and capture a command
`
	fmt.Fprint(w, help)
}

func printUnknownSubcommand(name string) {
	fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n\n", name)
	printHelp(os.Stderr)
}
