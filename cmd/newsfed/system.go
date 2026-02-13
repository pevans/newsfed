package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/pevans/newsfed/config"
	"github.com/pevans/newsfed/newsfeed"
	"github.com/pevans/newsfed/sources"
)

// loadStorageConfig loads storage configuration with precedence:
// 1. Environment variables (highest priority)
// 2. Configuration file (~/.newsfed/config.yaml)
// 3. Default values (lowest priority)
func loadStorageConfig() (metadataType, metadataPath, feedType, feedDir string) {
	// Set defaults
	metadataType = "sqlite"
	metadataPath = "metadata.db"
	feedType = "file"
	feedDir = ".news"

	// Load config file (if it exists)
	cfg, err := config.LoadConfigFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config file: %v\n", err)
		fmt.Fprintf(os.Stderr, "Continuing with defaults and environment variables...\n\n")
	}

	// Apply config file values (if loaded)
	if cfg != nil {
		if cfg.Storage.Metadata.Type != "" {
			metadataType = cfg.Storage.Metadata.Type
		}
		if cfg.Storage.Metadata.DSN != "" {
			metadataPath = cfg.Storage.Metadata.DSN
		}
		if cfg.Storage.Feed.Type != "" {
			feedType = cfg.Storage.Feed.Type
		}
		if cfg.Storage.Feed.DSN != "" {
			feedDir = cfg.Storage.Feed.DSN
		}
	}

	// Apply environment variables (highest priority)
	if val := os.Getenv("NEWSFED_METADATA_TYPE"); val != "" {
		metadataType = val
	}
	if val := os.Getenv("NEWSFED_METADATA_DSN"); val != "" {
		metadataPath = val
	}
	if val := os.Getenv("NEWSFED_FEED_TYPE"); val != "" {
		feedType = val
	}
	if val := os.Getenv("NEWSFED_FEED_DSN"); val != "" {
		feedDir = val
	}

	return metadataType, metadataPath, feedType, feedDir
}

func handleInit(metadataPath, feedDir string, args []string) {
	// Parse flags for init command
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	force := fs.Bool("force", false, "Force reinitialization even if storage already exists")
	fs.Parse(args)

	fmt.Println("Initializing newsfed storage...")
	fmt.Println()

	initSucceeded := true
	createdSomething := false

	// Create default config file as the first step
	created, err := config.WriteDefaultConfigFile(*force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ✗ Failed to create config file: %v\n", err)
		initSucceeded = false
	} else if created {
		configPath, _ := config.ConfigFilePath()
		fmt.Printf("  ✓ Config file: %s\n", configPath)
		createdSomething = true

		// Re-resolve storage paths from the new config file so that
		// subsequent storage creation uses the absolute paths
		if os.Getenv("NEWSFED_METADATA_DSN") == "" || os.Getenv("NEWSFED_FEED_DSN") == "" {
			_, newMeta, _, newFeed := loadStorageConfig()
			if os.Getenv("NEWSFED_METADATA_DSN") == "" {
				metadataPath = newMeta
			}
			if os.Getenv("NEWSFED_FEED_DSN") == "" {
				feedDir = newFeed
			}
		}
	} else {
		configPath, _ := config.ConfigFilePath()
		fmt.Printf("  Config file: %s (already exists)\n", configPath)
	}

	// Check and create metadata database
	metadataExists := false
	if _, err := os.Stat(metadataPath); err == nil {
		metadataExists = true
	}

	if metadataExists && !*force {
		fmt.Printf("  Metadata database: %s (already exists)\n", metadataPath)
	} else {
		// Create directory if needed
		metadataDir := ""
		if strings.Contains(metadataPath, "/") {
			lastSlash := strings.LastIndex(metadataPath, "/")
			metadataDir = metadataPath[:lastSlash]
		}

		if metadataDir != "" {
			if err := os.MkdirAll(metadataDir, 0o700); err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ Failed to create metadata directory: %v\n", err)
				initSucceeded = false
			}
		}

		// Initialize metadata database
		if initSucceeded {
			metadataStore, err := sources.NewSourceStore(metadataPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ Failed to initialize metadata database: %v\n", err)
				initSucceeded = false
			} else {
				metadataStore.Close()
				fmt.Printf("  ✓ Metadata database: %s\n", metadataPath)
				createdSomething = true
			}
		}

		// Initialize config table in metadata database
		if initSucceeded {
			configStore, err := config.NewConfigStore(metadataPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ Failed to initialize config table: %v\n", err)
				initSucceeded = false
			} else {
				configStore.Close()
			}
		}
	}

	// Check and create feed storage directory
	feedExists := false
	if stat, err := os.Stat(feedDir); err == nil && stat.IsDir() {
		feedExists = true
	}

	if feedExists && !*force {
		fmt.Printf("  Feed storage: %s (already exists)\n", feedDir)
	} else {
		// Create feed storage directory with proper permissions (0700 per RFC
		// 8 section 8.1)
		if err := os.MkdirAll(feedDir, 0o700); err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ Failed to create feed storage directory: %v\n", err)
			initSucceeded = false
		} else {
			// Verify we can write to it by initializing the NewsFeed
			newsFeed, err := newsfeed.NewNewsFeed(feedDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ Failed to initialize feed storage: %v\n", err)
				initSucceeded = false
			} else {
				_ = newsFeed
				fmt.Printf("  ✓ Feed storage: %s\n", feedDir)
				createdSomething = true
			}
		}
	}

	fmt.Println()

	if !initSucceeded {
		fmt.Println("✗ Initialization failed")
		os.Exit(1)
	}

	if !createdSomething && !*force {
		fmt.Println("✓ Storage already initialized")
		fmt.Println()
		fmt.Println("Use 'newsfed doctor' to check storage health")
	} else {
		fmt.Println("✓ Storage initialized successfully")
		fmt.Println()
		fmt.Println("You can now:")
		fmt.Println("  - Add sources with 'newsfed sources add'")
		fmt.Println("  - Check storage health with 'newsfed doctor'")
	}
}

func handleDoctor(metadataPath, feedDir string, args []string) {
	// Parse flags for doctor command
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	verbose := fs.Bool("verbose", false, "Show detailed diagnostic information")
	fs.Parse(args)

	fmt.Println("Checking newsfed storage health...")
	fmt.Println()

	hasErrors := false
	hasWarnings := false

	// Check metadata database
	fmt.Println("Metadata Database:")
	fmt.Printf("  Path: %s\n", metadataPath)

	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		fmt.Println("  ✗ Database file does not exist")
		fmt.Println("    Run 'newsfed init' to create it")
		hasErrors = true
	} else if err != nil {
		fmt.Printf("  ✗ Cannot access database file: %v\n", err)
		hasErrors = true
	} else {
		// Try to open the database
		metadataStore, err := sources.NewSourceStore(metadataPath)
		if err != nil {
			fmt.Printf("  ✗ Failed to open database: %v\n", err)
			hasErrors = true
		} else {
			defer metadataStore.Close()
			fmt.Println("  ✓ Database is accessible")

			// Check permissions
			if stat, err := os.Stat(metadataPath); err == nil {
				perm := stat.Mode().Perm()
				if *verbose {
					fmt.Printf("  Permissions: %o\n", perm)
				}
				// Database files should be 0600 (owner read/write only)
				if perm&0o077 != 0 {
					fmt.Println("  ⚠ Warning: Database file has overly permissive permissions")
					fmt.Printf("    Current: %o, expected: 600\n", perm)
					fmt.Println("    Consider: chmod 600 " + metadataPath)
					hasWarnings = true
				}
			}

			// Count sources
			sourceList, err := metadataStore.ListSources(sources.SourceFilter{})
			if err != nil {
				fmt.Printf("  ⚠ Warning: Could not list sources: %v\n", err)
				hasWarnings = true
			} else if *verbose || len(sourceList) > 0 {
				fmt.Printf("  Sources configured: %d\n", len(sourceList))
			}
		}
	}

	fmt.Println()

	// Check feed storage
	fmt.Println("Feed Storage:")
	fmt.Printf("  Path: %s\n", feedDir)

	if stat, err := os.Stat(feedDir); os.IsNotExist(err) {
		fmt.Println("  ✗ Storage directory does not exist")
		fmt.Println("    Run 'newsfed init' to create it")
		hasErrors = true
	} else if err != nil {
		fmt.Printf("  ✗ Cannot access storage directory: %v\n", err)
		hasErrors = true
	} else if !stat.IsDir() {
		fmt.Println("  ✗ Path exists but is not a directory")
		hasErrors = true
	} else {
		// Try to initialize the feed storage
		newsFeed, err := newsfeed.NewNewsFeed(feedDir)
		if err != nil {
			fmt.Printf("  ✗ Failed to initialize feed storage: %v\n", err)
			hasErrors = true
		} else {
			fmt.Println("  ✓ Storage directory is accessible")

			// Check permissions
			perm := stat.Mode().Perm()
			if *verbose {
				fmt.Printf("  Permissions: %o\n", perm)
			}
			// Storage directories should be 0700 (owner only)
			if perm&0o077 != 0 {
				fmt.Println("  ⚠ Warning: Storage directory has overly permissive permissions")
				fmt.Printf("    Current: %o, expected: 700\n", perm)
				fmt.Println("    Consider: chmod 700 " + feedDir)
				hasWarnings = true
			}

			// Check individual feed file permissions
			entries, dirErr := os.ReadDir(feedDir)
			if dirErr != nil {
				fmt.Printf("  ⚠ Warning: Could not read feed directory: %v\n", dirErr)
				hasWarnings = true
			} else {
				looseFileCount := 0
				for _, entry := range entries {
					if entry.IsDir() {
						continue
					}
					info, err := entry.Info()
					if err != nil {
						continue
					}
					filePerm := info.Mode().Perm()
					if filePerm&0o077 != 0 {
						looseFileCount++
					}
				}
				if looseFileCount > 0 {
					fmt.Printf("  ⚠ Warning: %d file(s) have overly permissive permissions\n", looseFileCount)
					fmt.Printf("    Consider: chmod 600 %s/*\n", feedDir)
					hasWarnings = true
				}
			}

			// Count items
			result, err := newsFeed.List()
			if err != nil {
				fmt.Printf("  ⚠ Warning: Could not list items: %v\n", err)
				hasWarnings = true
			} else {
				if *verbose || len(result.Items) > 0 {
					fmt.Printf("  News items stored: %d\n", len(result.Items))
				}
				if len(result.Errors) > 0 {
					fmt.Printf("  ⚠ Warning: %d item(s) could not be read\n", len(result.Errors))
					hasWarnings = true
				}
			}
		}
	}

	fmt.Println()

	// Print summary
	if hasErrors {
		fmt.Println("✗ Storage has errors")
		fmt.Println("  Run 'newsfed init' to initialize storage")
		os.Exit(1)
	} else if hasWarnings {
		fmt.Println("✓ Storage is functional but has warnings")
		if !*verbose {
			fmt.Println("  Run 'newsfed doctor -verbose' for more details")
		}
	} else {
		fmt.Println("✓ All checks passed")
	}
}
