package scraper

// ScraperConfig defines how to extract articles from a specific website.
// Implements Spec 3 section 2.2.
type ScraperConfig struct {
	DiscoveryMode string        `json:"discovery_mode"` // "list" or "direct"
	ListConfig    *ListConfig   `json:"list_config,omitempty"`
	ArticleConfig ArticleConfig `json:"article_config"`
}

// ListConfig defines how to discover articles from listing/index pages. Used
// when DiscoveryMode is "list". Implements Spec 3 section 2.2.
type ListConfig struct {
	ArticleSelector    string `json:"article_selector"`
	PaginationSelector string `json:"pagination_selector,omitempty"`
	MaxPages           int    `json:"max_pages"` // Default: 1
}

// ArticleConfig defines how to extract metadata from individual article
// pages. Implements Spec 3 section 2.2.
type ArticleConfig struct {
	TitleSelector   string `json:"title_selector"`
	ContentSelector string `json:"content_selector"`
	AuthorSelector  string `json:"author_selector,omitempty"`
	DateSelector    string `json:"date_selector,omitempty"`
	DateFormat      string `json:"date_format,omitempty"` // Go time format string
}

// NewListConfig creates a new list configuration with default values.
func NewListConfig(articleSelector string) *ListConfig {
	return &ListConfig{
		ArticleSelector: articleSelector,
		MaxPages:        1, // Default per Spec 3 section 2.2
	}
}
