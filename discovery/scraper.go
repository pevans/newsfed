package discovery

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
	"github.com/pevans/newsfed/newsfeed"
	"github.com/pevans/newsfed/scraper"
)

// Re-export types for backward compatibility
type (
	ScraperConfig = scraper.ScraperConfig
	ListConfig    = scraper.ListConfig
	ArticleConfig = scraper.ArticleConfig
)

// NewListConfig creates a new list configuration with default values.
func NewListConfig(articleSelector string) *scraper.ListConfig {
	return scraper.NewListConfig(articleSelector)
}

// ScraperSource represents a web scraping source configuration. Implements
// Spec 3 section 2.1.
type ScraperSource struct {
	SourceID      uuid.UUID              `json:"source_id"`
	SourceType    string                 `json:"source_type"` // Always "website"
	URL           string                 `json:"url"`
	Name          string                 `json:"name"`
	Enabled       bool                   `json:"enabled"`
	ScraperConfig *scraper.ScraperConfig `json:"scraper_config"`
}

// NewScraperSource creates a new scraper source with the given parameters.
func NewScraperSource(url, name string, config *scraper.ScraperConfig) *ScraperSource {
	return &ScraperSource{
		SourceID:      uuid.New(),
		SourceType:    "website",
		URL:           url,
		Name:          name,
		Enabled:       true,
		ScraperConfig: config,
	}
}

// ScrapedArticle holds extracted article data from a web page before
// conversion to NewsItem.
type ScrapedArticle struct {
	Title       string
	Content     string
	URL         string
	Authors     []string
	PublishedAt *time.Time
}

// ScrapedArticleToNewsItem converts scraped article data to a NewsItem.
// Implements Spec 3 section 4.1 field mapping.
func ScrapedArticleToNewsItem(article *ScrapedArticle, publisherName string) newsfeed.NewsItem {
	// Generate new UUID for the item
	id := uuid.New()

	// Title: use extracted title, fall back to "(No title)" if empty
	title := article.Title
	if title == "" {
		title = "(No title)"
	}

	// Summary: truncate content to reasonable length (500 chars per Spec 3
	// section 3.4)
	summary := article.Content
	if len(summary) > 500 {
		summary = summary[:500] + "..."
	}

	// URL: from the article page URL
	url := article.URL

	// Publisher: from source-level name field
	var publisher *string
	if publisherName != "" {
		publisher = &publisherName
	}

	// Authors: from extracted authors
	authors := article.Authors
	if authors == nil {
		authors = []string{}
	}

	// Published_at: from extracted date or fallback to current time
	var publishedAt time.Time
	if article.PublishedAt != nil {
		publishedAt = *article.PublishedAt
	} else {
		publishedAt = time.Now()
	}

	// Discovered_at: set to current time when ingesting
	discoveredAt := time.Now()

	// Pinned_at: set to nil (not yet pinned)
	var pinnedAt *time.Time

	return newsfeed.NewsItem{
		ID:           id,
		Title:        title,
		Summary:      summary,
		URL:          url,
		Publisher:    publisher,
		Authors:      authors,
		PublishedAt:  publishedAt,
		DiscoveredAt: discoveredAt,
		PinnedAt:     pinnedAt,
	}
}

// ParseAuthors splits a single author string into multiple authors if it
// contains common delimiters. Implements Spec 3 section 3.4.
func ParseAuthors(authorText string) []string {
	if authorText == "" {
		return []string{}
	}

	// Split on common delimiters: ", " or " and "
	authors := []string{}

	// Try splitting by ", " first
	if strings.Contains(authorText, ", ") {
		parts := strings.SplitSeq(authorText, ", ")
		for part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				authors = append(authors, part)
			}
		}
		return authors
	}

	// Try splitting by " and "
	if strings.Contains(authorText, " and ") {
		parts := strings.SplitSeq(authorText, " and ")
		for part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				authors = append(authors, part)
			}
		}
		return authors
	}

	// No delimiters found, return as single author
	return []string{strings.TrimSpace(authorText)}
}

// URLExists checks if a NewsItem with the given URL already exists in the
// feed. Implements Spec 3 section 4.2 deduplication strategy.
func URLExists(feed *newsfeed.NewsFeed, url string) (bool, error) {
	result, err := feed.List()
	if err != nil {
		return false, err
	}

	for _, item := range result.Items {
		if item.URL == url {
			return true, nil
		}
	}

	return false, nil
}

// FetchHTML fetches HTML content from the given URL. Implements Spec 3
// section 3.2.
func FetchHTML(url string) (*goquery.Document, error) {
	// Create HTTP client with 10 second timeout per Spec 3 section 3.2
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set User-Agent header identifying newsfed per Spec 3 section 3.2
	req.Header.Set("User-Agent", "newsfed/1.0 (RSS/Atom aggregator with web scraping)")

	// Perform the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	// Parse HTML with goquery
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	return doc, nil
}

// ExtractArticle extracts article data from HTML using the given selectors.
// Implements Spec 3 section 3.4.
func ExtractArticle(doc *goquery.Document, config scraper.ArticleConfig, articleURL string) (*ScrapedArticle, error) {
	article := &ScrapedArticle{
		URL: articleURL,
	}

	// Extract title (required)
	titleText := doc.Find(config.TitleSelector).First().Text()
	// Normalize whitespace: replace multiple spaces/newlines with single space
	titleText = strings.Join(strings.Fields(titleText), " ")
	if titleText == "" {
		titleText = "(No title)"
	}
	article.Title = titleText

	// Extract content (required)
	contentText := strings.TrimSpace(doc.Find(config.ContentSelector).First().Text())
	// Normalize whitespace: replace multiple spaces/newlines with single
	// space
	contentText = strings.Join(strings.Fields(contentText), " ")
	article.Content = contentText

	// Extract authors (optional)
	if config.AuthorSelector != "" {
		authors := []string{}
		doc.Find(config.AuthorSelector).Each(func(i int, s *goquery.Selection) {
			authorText := strings.TrimSpace(s.Text())
			if authorText != "" {
				// Parse for multiple authors in single element
				parsed := ParseAuthors(authorText)
				authors = append(authors, parsed...)
			}
		})
		article.Authors = authors
	}

	// Extract published date (optional)
	if config.DateSelector != "" && config.DateFormat != "" {
		dateText := strings.TrimSpace(doc.Find(config.DateSelector).First().Text())
		if dateText != "" {
			// Try to parse the date using the provided format
			publishedAt, err := time.Parse(config.DateFormat, dateText)
			if err == nil {
				article.PublishedAt = &publishedAt
			}
			// If parsing fails, PublishedAt remains nil (fallback to current
			// time in ScrapedArticleToNewsItem)
		}
	}

	return article, nil
}

// ScrapeArticle is a convenience function that fetches and extracts an
// article in one call. Combines FetchHTML and ExtractArticle.
func ScrapeArticle(url string, config scraper.ArticleConfig) (*ScrapedArticle, error) {
	// Fetch HTML
	doc, err := FetchHTML(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch HTML: %w", err)
	}

	// Extract article data
	article, err := ExtractArticle(doc, config, url)
	if err != nil {
		return nil, fmt.Errorf("failed to extract article: %w", err)
	}

	return article, nil
}

// ValidateScrapedArticle validates a scraped article before storing.
// Implements Spec 3 section 6.3.
func ValidateScrapedArticle(article *ScrapedArticle, sourceURL string) error {
	// Validate title: must be non-empty and reasonable length
	if article.Title == "" {
		return fmt.Errorf("title is empty")
	}
	if len(article.Title) > 500 {
		return fmt.Errorf("title too long (%d characters, max 500)", len(article.Title))
	}

	// Validate URL: must be valid
	articleURL, err := url.Parse(article.URL)
	if err != nil {
		return fmt.Errorf("invalid article URL: %w", err)
	}
	if articleURL.Scheme != "http" && articleURL.Scheme != "https" {
		return fmt.Errorf("article URL must use http or https scheme")
	}

	// Validate URL: must point to same domain as source
	sourceURLParsed, err := url.Parse(sourceURL)
	if err != nil {
		return fmt.Errorf("invalid source URL: %w", err)
	}
	if articleURL.Host != sourceURLParsed.Host {
		return fmt.Errorf("article URL domain (%s) does not match source domain (%s)",
			articleURL.Host, sourceURLParsed.Host)
	}

	// Validate summary/content: warn if empty but don't reject
	if article.Content == "" {
		fmt.Fprintf(os.Stderr, "Warning: article has empty content (URL: %s)\n", article.URL)
	}

	// Validate published date: must be reasonable if present
	if article.PublishedAt != nil {
		// Minimum date: 1990-01-01 per Spec 3 section 6.3
		minDate := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
		if article.PublishedAt.Before(minDate) {
			return fmt.Errorf("published date (%s) is before minimum date (1990-01-01)",
				article.PublishedAt.Format("2006-01-02"))
		}

		// Must not be in the future
		if article.PublishedAt.After(time.Now()) {
			return fmt.Errorf("published date (%s) is in the future",
				article.PublishedAt.Format("2006-01-02"))
		}
	}

	return nil
}
