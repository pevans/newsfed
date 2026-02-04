package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/pevans/newsfed"
)

// getEnv returns the value of an environment variable or a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Parse global flags
	metadataPath := getEnv("NEWSFED_METADATA_DSN", "metadata.db")

	// Get subcommand
	subcommand := os.Args[1]

	switch subcommand {
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
	// Initialize metadata store
	metadataStore, err := newsfed.NewMetadataStore(metadataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open metadata store: %v\n", err)
		os.Exit(1)
	}
	defer metadataStore.Close()

	switch action {
	case "list":
		handleSourcesList(metadataStore, args)
	case "add":
		handleSourcesAdd(metadataStore, args)
	case "delete":
		handleSourcesDelete(metadataStore, args)
	case "help", "--help", "-h":
		printSourcesUsage()
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown sources command: %s\n\n", action)
		printSourcesUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("newsfed - News feed CLI client")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  newsfed <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  sources    Manage news sources")
	fmt.Println("  help       Show this help message")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  NEWSFED_METADATA_DSN  Path to metadata database (default: metadata.db)")
	fmt.Println("  NEWSFED_FEED_DSN      Path to news feed storage (default: .news)")
}

func printSourcesUsage() {
	fmt.Println("newsfed sources - Manage news sources")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  newsfed sources <action> [arguments]")
	fmt.Println()
	fmt.Println("Actions:")
	fmt.Println("  list       List all sources")
	fmt.Println("  add        Add a new source")
	fmt.Println("  delete     Delete a source")
	fmt.Println("  help       Show this help message")
}

func handleSourcesList(metadataStore *newsfed.MetadataStore, args []string) {
	// Parse flags for list command
	fs := flag.NewFlagSet("sources list", flag.ExitOnError)
	fs.Parse(args)

	// Get all sources
	sources, err := metadataStore.ListSources()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to list sources: %v\n", err)
		os.Exit(1)
	}

	if len(sources) == 0 {
		fmt.Println("No sources configured.")
		return
	}

	// Print table header
	fmt.Printf("%-36s %-10s %-50s %s\n", "ID", "TYPE", "NAME", "URL")
	fmt.Println("----------------------------------------------------------------------------------------------------")

	// Print each source
	for _, source := range sources {
		// Truncate name and URL if too long
		name := source.Name
		if len(name) > 50 {
			name = name[:47] + "..."
		}
		url := source.URL
		if len(url) > 50 {
			url = url[:47] + "..."
		}

		fmt.Printf("%-36s %-10s %-50s %s\n",
			source.SourceID.String(),
			source.SourceType,
			name,
			url,
		)
	}
}

func handleSourcesAdd(metadataStore *newsfed.MetadataStore, args []string) {
	// Parse flags for add command
	fs := flag.NewFlagSet("sources add", flag.ExitOnError)
	sourceType := fs.String("type", "", "Source type (rss or atom)")
	url := fs.String("url", "", "Source URL")
	name := fs.String("name", "", "Source name")
	fs.Parse(args)

	// Validate required flags
	if *sourceType == "" {
		fmt.Fprintf(os.Stderr, "Error: --type is required\n")
		fs.Usage()
		os.Exit(1)
	}
	if *url == "" {
		fmt.Fprintf(os.Stderr, "Error: --url is required\n")
		fs.Usage()
		os.Exit(1)
	}
	if *name == "" {
		fmt.Fprintf(os.Stderr, "Error: --name is required\n")
		fs.Usage()
		os.Exit(1)
	}

	// Validate source type
	if *sourceType != "rss" && *sourceType != "atom" {
		fmt.Fprintf(os.Stderr, "Error: --type must be 'rss' or 'atom'\n")
		os.Exit(1)
	}

	// Create the source (enabled by default)
	now := time.Now()
	source, err := metadataStore.CreateSource(*sourceType, *url, *name, nil, &now)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create source: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Created source: %s\n", source.SourceID.String())
	fmt.Printf("  Type: %s\n", source.SourceType)
	fmt.Printf("  Name: %s\n", source.Name)
	fmt.Printf("  URL: %s\n", source.URL)
}

func handleSourcesDelete(metadataStore *newsfed.MetadataStore, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: source ID is required\n")
		fmt.Fprintf(os.Stderr, "Usage: newsfed sources delete <source-id>\n")
		os.Exit(1)
	}

	sourceID := args[0]

	// Parse UUID
	id, err := uuid.Parse(sourceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid source ID: %v\n", err)
		os.Exit(1)
	}

	// Delete the source
	err = metadataStore.DeleteSource(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to delete source: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Deleted source: %s\n", sourceID)
}
