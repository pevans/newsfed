package main

import (
	"fmt"
	"os"

	"github.com/pevans/newsfed/sources"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Load storage configuration with precedence: env vars > config file > defaults
	metadataType, metadataPath, feedType, feedDir := loadStorageConfig()

	// Validate storage types (currently only sqlite and file are supported)
	if metadataType != "sqlite" {
		fmt.Fprintf(os.Stderr, "Error: unsupported metadata storage type: %s\n", metadataType)
		fmt.Fprintf(os.Stderr, "Supported types: sqlite\n")
		os.Exit(1)
	}
	if feedType != "file" {
		fmt.Fprintf(os.Stderr, "Error: unsupported feed storage type: %s\n", feedType)
		fmt.Fprintf(os.Stderr, "Supported types: file\n")
		os.Exit(1)
	}

	// Get subcommand
	subcommand := os.Args[1]

	switch subcommand {
	case "list":
		handleList(feedDir, os.Args[2:])
	case "show":
		handleShow(feedDir, os.Args[2:])
	case "pin":
		handlePin(feedDir, os.Args[2:])
	case "unpin":
		handleUnpin(feedDir, os.Args[2:])
	case "open":
		handleOpen(metadataPath, feedDir, os.Args[2:])
	case "prune":
		handlePrune(feedDir, os.Args[2:])
	case "sync":
		handleSync(metadataPath, feedDir, os.Args[2:])
	case "init":
		handleInit(metadataPath, feedDir, os.Args[2:])
	case "doctor":
		handleDoctor(metadataPath, feedDir, os.Args[2:])
	case "sources":
		if len(os.Args) < 3 {
			printSourcesUsage()
			os.Exit(1)
		}
		action := os.Args[2]
		handleSourcesCommand(action, metadataPath, os.Args[3:])
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command: %s\n\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func handleSourcesCommand(action, metadataPath string, args []string) {
	// Initialize source store
	sourceStore, err := sources.NewSourceStore(metadataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open source store: %v\n", err)
		os.Exit(1)
	}
	defer sourceStore.Close()

	switch action {
	case "list":
		handleSourcesList(sourceStore, args)
	case "show":
		handleSourcesShow(sourceStore, args)
	case "add":
		handleSourcesAdd(sourceStore, args)
	case "update":
		handleSourcesUpdate(sourceStore, args)
	case "delete":
		handleSourcesDelete(sourceStore, args)
	case "enable":
		handleSourcesEnable(sourceStore, args)
	case "disable":
		handleSourcesDisable(sourceStore, args)
	case "status":
		handleSourcesStatus(sourceStore, args)
	case "errors":
		handleSourcesErrors(sourceStore, args)
	case "help", "--help", "-h":
		printSourcesUsage()
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown sources command: %s\n\n", action)
		printSourcesUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("newsfed -- News feed CLI client")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  newsfed <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  list       List news items")
	fmt.Println("  show       Show detailed view of a news item")
	fmt.Println("  pin        Pin a news item for later reference")
	fmt.Println("  unpin      Unpin a news item")
	fmt.Println("  open       Open a news item URL in default browser")
	fmt.Println("  prune      Remove stale news items")
	fmt.Println("  sync       Manually sync sources to fetch new items")
	fmt.Println("  init       Initialize storage (create databases/directories)")
	fmt.Println("  doctor     Check storage health and configuration")
	fmt.Println("  sources    Manage news sources")
	fmt.Println("  help       Show this help message")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  NEWSFED_METADATA_TYPE  Metadata storage type (default: sqlite)")
	fmt.Println("  NEWSFED_METADATA_DSN   Path to metadata database (default: metadata.db)")
	fmt.Println("  NEWSFED_FEED_TYPE      Feed storage type (default: file)")
	fmt.Println("  NEWSFED_FEED_DSN       Path to news feed storage (default: .news)")
}
