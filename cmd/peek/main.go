package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

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

func handleMCP(args []string) {
	fmt.Fprintf(os.Stderr, "mcp: not yet implemented\n")
	os.Exit(1)
}

func handleList(args []string) {
	fmt.Fprintf(os.Stderr, "list: not yet implemented\n")
	os.Exit(1)
}

func handleLogs(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "logs: missing id argument\n")
		os.Exit(2)
	}
	id := args[0]
	fmt.Fprintf(os.Stderr, "logs: not yet implemented (id=%s)\n", id)
	os.Exit(1)
}

func handleWrap(args []string) {
	cmd := args[0]
	var wrappedArgs string
	if len(args) > 1 {
		wrappedArgs = strings.Join(args[1:], " ")
		fmt.Fprintf(os.Stderr, "wrap: not yet implemented (cmd=%s args=%s)\n", cmd, wrappedArgs)
	} else {
		fmt.Fprintf(os.Stderr, "wrap: not yet implemented (cmd=%s args=)\n", cmd)
	}
	os.Exit(1)
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
