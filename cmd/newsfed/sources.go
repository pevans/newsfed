package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/pevans/newsfed/discovery"
	"github.com/pevans/newsfed/sources"
)

func printSourcesUsage() {
	fmt.Println("newsfed sources -- Manage news sources")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  newsfed sources <action> [arguments]")
	fmt.Println()
	fmt.Println("Actions:")
	fmt.Println("  list       List all sources")
	fmt.Println("  show       Show detailed source information")
	fmt.Println("  add        Add a new source")
	fmt.Println("  update     Update source configuration")
	fmt.Println("  delete     Delete a source")
	fmt.Println("  enable     Enable a source")
	fmt.Println("  disable    Disable a source")
	fmt.Println("  status     Check source health")
	fmt.Println("  help       Show this help message")
}

func handleSourcesList(metadataStore *sources.SourceStore, args []string) {
	// Parse flags for list command
	fs := flag.NewFlagSet("sources list", flag.ExitOnError)
	fs.Parse(args)

	// Get all sources
	sourceList, err := metadataStore.ListSources(sources.SourceFilter{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to list sources: %v\n", err)
		os.Exit(1)
	}

	if len(sourceList) == 0 {
		fmt.Println("No sources configured.")
		return
	}

	// Print table header
	fmt.Printf("%-36s %-10s %-50s %s\n", "ID", "TYPE", "NAME", "URL")
	fmt.Println("----------------------------------------------------------------------------------------------------")

	// Print each source
	for _, source := range sourceList {
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

func handleSourcesShow(metadataStore *sources.SourceStore, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: source ID is required\n")
		fmt.Fprintf(os.Stderr, "Usage: newsfed sources show <source-id>\n")
		os.Exit(1)
	}

	sourceID := args[0]

	// Parse UUID
	id, err := uuid.Parse(sourceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid source ID: %v\n", err)
		os.Exit(1)
	}

	// Get the source
	source, err := metadataStore.GetSource(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get source: %v\n", err)
		os.Exit(1)
	}

	// Display the source
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println(source.Name)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// Basic info
	fmt.Printf("Type:        %s\n", source.SourceType)
	fmt.Printf("URL:         %s\n", source.URL)
	fmt.Println()

	// Status
	if source.EnabledAt != nil {
		fmt.Printf("Status:      ✓ Enabled (since %s)\n", source.EnabledAt.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Println("Status:      ✗ Disabled")
	}
	fmt.Println()

	// Operational metadata
	fmt.Println("Operational Info:")
	if source.LastFetchedAt != nil {
		fmt.Printf("  Last Fetched:    %s\n", source.LastFetchedAt.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Println("  Last Fetched:    Never")
	}

	if source.PollingInterval != nil {
		fmt.Printf("  Poll Interval:   %s\n", *source.PollingInterval)
	} else {
		fmt.Println("  Poll Interval:   Default")
	}
	fmt.Println()

	// Health status
	fmt.Println("Health:")
	fmt.Printf("  Error Count:     %d\n", source.FetchErrorCount)
	if source.LastError != nil {
		fmt.Printf("  Last Error:      %s\n", *source.LastError)
	} else {
		fmt.Println("  Last Error:      None")
	}
	fmt.Println()

	// HTTP cache headers
	if source.LastModified != nil || source.ETag != nil {
		fmt.Println("HTTP Cache:")
		if source.LastModified != nil {
			fmt.Printf("  Last-Modified:   %s\n", *source.LastModified)
		}
		if source.ETag != nil {
			fmt.Printf("  ETag:            %s\n", *source.ETag)
		}
		fmt.Println()
	}

	// Scraper config (for website sources)
	if source.ScraperConfig != nil {
		fmt.Println("Scraper Configuration:")
		fmt.Printf("  Discovery Mode:     %s\n", source.ScraperConfig.DiscoveryMode)
		if source.ScraperConfig.ListConfig != nil {
			fmt.Printf("  Article Selector:   %s\n", source.ScraperConfig.ListConfig.ArticleSelector)
			if source.ScraperConfig.ListConfig.PaginationSelector != "" {
				fmt.Printf("  Pagination:         %s\n", source.ScraperConfig.ListConfig.PaginationSelector)
			}
			fmt.Printf("  Max Pages:          %d\n", source.ScraperConfig.ListConfig.MaxPages)
		}
		fmt.Printf("  Title Selector:     %s\n", source.ScraperConfig.ArticleConfig.TitleSelector)
		fmt.Printf("  Content Selector:   %s\n", source.ScraperConfig.ArticleConfig.ContentSelector)
		if source.ScraperConfig.ArticleConfig.AuthorSelector != "" {
			fmt.Printf("  Author Selector:    %s\n", source.ScraperConfig.ArticleConfig.AuthorSelector)
		}
		if source.ScraperConfig.ArticleConfig.DateSelector != "" {
			fmt.Printf("  Date Selector:      %s\n", source.ScraperConfig.ArticleConfig.DateSelector)
		}
		fmt.Println()
	}

	// Dates
	fmt.Printf("Created:     %s\n", source.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated:     %s\n", source.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()

	// ID
	fmt.Printf("ID:          %s\n", source.SourceID.String())
}

func handleSourcesAdd(metadataStore *sources.SourceStore, args []string) {
	// Parse flags for add command
	fs := flag.NewFlagSet("sources add", flag.ExitOnError)
	sourceType := fs.String("type", "", "Source type (rss, atom, or website)")
	url := fs.String("url", "", "Source URL")
	name := fs.String("name", "", "Source name")
	configFile := fs.String("config", "", "Scraper config file (for website sources)")
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
	if *sourceType != "rss" && *sourceType != "atom" && *sourceType != "website" {
		fmt.Fprintf(os.Stderr, "Error: --type must be 'rss', 'atom', or 'website'\n")
		os.Exit(1)
	}

	// For website sources, config is required
	var scraperConfig *discovery.ScraperConfig
	if *sourceType == "website" {
		if *configFile == "" {
			fmt.Fprintf(os.Stderr, "Error: --config is required for website sources\n")
			os.Exit(1)
		}

		// Read and parse config file
		data, err := os.ReadFile(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to read config file: %v\n", err)
			os.Exit(1)
		}

		scraperConfig = &discovery.ScraperConfig{}
		if err := json.Unmarshal(data, scraperConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to parse config file: %v\n", err)
			os.Exit(1)
		}
	}

	// Create the source (enabled by default)
	now := time.Now()
	source, err := metadataStore.CreateSource(*sourceType, *url, *name, scraperConfig, &now)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create source: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Created source: %s\n", source.SourceID.String())
	fmt.Printf("  Type: %s\n", source.SourceType)
	fmt.Printf("  Name: %s\n", source.Name)
	fmt.Printf("  URL: %s\n", source.URL)
	if scraperConfig != nil {
		fmt.Println("  Scraper: Configured")
	}
}

func handleSourcesUpdate(metadataStore *sources.SourceStore, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: source ID is required\n")
		fmt.Fprintf(os.Stderr, "Usage: newsfed sources update <source-id> [flags]\n")
		os.Exit(1)
	}

	sourceID := args[0]

	// Parse UUID
	id, err := uuid.Parse(sourceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid source ID: %v\n", err)
		os.Exit(1)
	}

	// Parse flags for update command
	fs := flag.NewFlagSet("sources update", flag.ExitOnError)
	name := fs.String("name", "", "Update source name")
	interval := fs.String("interval", "", "Update polling interval (e.g., 30m, 1h)")
	configFile := fs.String("config", "", "Update scraper config file (for website sources)")
	fs.Parse(args[1:])

	// Check if any updates were provided
	if *name == "" && *interval == "" && *configFile == "" {
		fmt.Fprintf(os.Stderr, "Error: at least one update flag is required (--name, --interval, or --config)\n")
		os.Exit(1)
	}

	// Build updates struct
	update := sources.SourceUpdate{}

	if *name != "" {
		update.Name = name
	}

	if *interval != "" {
		// Validate interval format by parsing it
		_, err := parseDuration(*interval)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid interval format: %v\n", err)
			os.Exit(1)
		}
		update.PollingInterval = interval
	}

	if *configFile != "" {
		// Read and parse config file
		data, err := os.ReadFile(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to read config file: %v\n", err)
			os.Exit(1)
		}

		scraperConfig := &discovery.ScraperConfig{}
		if err := json.Unmarshal(data, scraperConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to parse config file: %v\n", err)
			os.Exit(1)
		}
		update.ScraperConfig = scraperConfig
	}

	// Apply updates
	err = metadataStore.UpdateSource(id, update)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to update source: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Updated source: %s\n", sourceID)
	if *name != "" {
		fmt.Printf("  Name: %s\n", *name)
	}
	if *interval != "" {
		fmt.Printf("  Interval: %s\n", *interval)
	}
	if *configFile != "" {
		fmt.Println("  Scraper: Updated")
	}
}

func handleSourcesDelete(metadataStore *sources.SourceStore, args []string) {
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

func handleSourcesEnable(metadataStore *sources.SourceStore, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: source ID is required\n")
		fmt.Fprintf(os.Stderr, "Usage: newsfed sources enable <source-id>\n")
		os.Exit(1)
	}

	sourceID := args[0]

	// Parse UUID
	id, err := uuid.Parse(sourceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid source ID: %v\n", err)
		os.Exit(1)
	}

	// Get the source
	source, err := metadataStore.GetSource(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get source: %v\n", err)
		os.Exit(1)
	}

	// Check if already enabled
	if source.EnabledAt != nil {
		fmt.Printf("Source is already enabled (enabled at: %s)\n", source.EnabledAt.Format("2006-01-02 15:04:05"))
		return
	}

	// Enable the source
	now := time.Now()
	update := sources.SourceUpdate{
		EnabledAt: &now,
	}

	err = metadataStore.UpdateSource(id, update)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to enable source: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Enabled source: %s\n", source.Name)
}

func handleSourcesDisable(metadataStore *sources.SourceStore, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: source ID is required\n")
		fmt.Fprintf(os.Stderr, "Usage: newsfed sources disable <source-id>\n")
		os.Exit(1)
	}

	sourceID := args[0]

	// Parse UUID
	id, err := uuid.Parse(sourceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid source ID: %v\n", err)
		os.Exit(1)
	}

	// Get the source
	source, err := metadataStore.GetSource(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get source: %v\n", err)
		os.Exit(1)
	}

	// Check if already disabled
	if source.EnabledAt == nil {
		fmt.Println("Source is already disabled")
		return
	}

	// Disable the source
	update := sources.SourceUpdate{
		ClearEnabledAt: true,
	}

	err = metadataStore.UpdateSource(id, update)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to disable source: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Disabled source: %s\n", source.Name)
}

func handleSourcesStatus(metadataStore *sources.SourceStore, args []string) {
	// Parse flags
	fs := flag.NewFlagSet("sources status", flag.ExitOnError)
	verbose := fs.Bool("verbose", false, "Show detailed error information")
	fs.Parse(args)

	// Get all sources to analyze
	allSources, err := metadataStore.ListSources(sources.SourceFilter{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to list sources: %v\n", err)
		os.Exit(1)
	}

	if len(allSources) == 0 {
		fmt.Println("No sources configured.")
		return
	}

	// Categorize sources by health status
	var (
		withErrors   []sources.Source
		neverFetched []sources.Source
		stale        []sources.Source // Not fetched in > 24 hours
		disabled     []sources.Source
		healthy      []sources.Source
	)

	now := time.Now()
	staleThreshold := 24 * time.Hour

	for _, source := range allSources {
		// Check if disabled
		if !source.IsEnabled() {
			disabled = append(disabled, source)
			continue
		}

		// Check if has errors
		if source.FetchErrorCount > 0 || source.LastError != nil {
			withErrors = append(withErrors, source)
			continue
		}

		// Check if never fetched
		if source.LastFetchedAt == nil {
			neverFetched = append(neverFetched, source)
			continue
		}

		// Check if stale
		if now.Sub(*source.LastFetchedAt) > staleThreshold {
			stale = append(stale, source)
			continue
		}

		// Otherwise healthy
		healthy = append(healthy, source)
	}

	// Print summary
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Source Health Status")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	fmt.Printf("✓ Healthy:          %d\n", len(healthy))
	fmt.Printf("⚠ With Errors:      %d\n", len(withErrors))
	fmt.Printf("⚠ Never Fetched:    %d\n", len(neverFetched))
	fmt.Printf("⚠ Stale (>24h):     %d\n", len(stale))
	fmt.Printf("✗ Disabled:         %d\n", len(disabled))
	fmt.Println()

	// If everything is healthy, we can stop here
	if len(withErrors) == 0 && len(neverFetched) == 0 && len(stale) == 0 && len(disabled) == 0 {
		fmt.Println("All sources are healthy!")
		return
	}

	// Print sources with errors
	if len(withErrors) > 0 {
		fmt.Println("━━━ Sources with Errors ━━━")
		fmt.Println()
		for _, source := range withErrors {
			fmt.Printf("⚠ %s\n", source.Name)
			fmt.Printf("  ID: %s\n", source.SourceID.String())
			fmt.Printf("  URL: %s\n", source.URL)
			fmt.Printf("  Error Count: %d\n", source.FetchErrorCount)
			if source.LastError != nil && *verbose {
				fmt.Printf("  Last Error: %s\n", *source.LastError)
			} else if source.LastError != nil {
				// Truncate error message if not verbose
				errMsg := *source.LastError
				if len(errMsg) > 80 {
					errMsg = errMsg[:77] + "..."
				}
				fmt.Printf("  Last Error: %s\n", errMsg)
			}
			if source.LastFetchedAt != nil {
				fmt.Printf("  Last Attempted: %s\n", source.LastFetchedAt.Format("2006-01-02 15:04:05"))
			}
			fmt.Println()
		}
	}

	// Print sources never fetched
	if len(neverFetched) > 0 {
		fmt.Println("━━━ Sources Never Fetched ━━━")
		fmt.Println()
		for _, source := range neverFetched {
			fmt.Printf("⚠ %s\n", source.Name)
			fmt.Printf("  ID: %s\n", source.SourceID.String())
			fmt.Printf("  URL: %s\n", source.URL)
			fmt.Printf("  Created: %s\n", source.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Println()
		}
	}

	// Print stale sources
	if len(stale) > 0 {
		fmt.Println("━━━ Stale Sources (>24h since fetch) ━━━")
		fmt.Println()
		for _, source := range stale {
			fmt.Printf("⚠ %s\n", source.Name)
			fmt.Printf("  ID: %s\n", source.SourceID.String())
			fmt.Printf("  URL: %s\n", source.URL)
			if source.LastFetchedAt != nil {
				elapsed := now.Sub(*source.LastFetchedAt)
				fmt.Printf("  Last Fetched: %s (%s ago)\n",
					source.LastFetchedAt.Format("2006-01-02 15:04:05"),
					formatDuration(elapsed))
			}
			fmt.Println()
		}
	}

	// Print disabled sources
	if len(disabled) > 0 {
		fmt.Println("━━━ Disabled Sources ━━━")
		fmt.Println()
		for _, source := range disabled {
			fmt.Printf("✗ %s\n", source.Name)
			fmt.Printf("  ID: %s\n", source.SourceID.String())
			fmt.Printf("  URL: %s\n", source.URL)
			if *verbose && source.LastError != nil {
				fmt.Printf("  Last Error: %s\n", *source.LastError)
			}
			fmt.Println()
		}
	}

	// Suggest actions
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Suggested Actions:")
	if len(withErrors) > 0 {
		fmt.Println("  • Check source configurations for errors")
		fmt.Println("  • Run 'newsfed sources show <id>' for details")
	}
	if len(neverFetched) > 0 {
		fmt.Println("  • Run 'newsfed sync' to fetch from all sources")
	}
	if len(stale) > 0 {
		fmt.Println("  • Check if polling daemon is running")
		fmt.Println("  • Run 'newsfed sync' to manually fetch")
	}
	if len(disabled) > 0 {
		fmt.Println("  • Run 'newsfed sources enable <id>' to re-enable")
	}
	fmt.Println()
}

// formatDuration formats a duration in human-readable form
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	return fmt.Sprintf("%dd", days)
}
