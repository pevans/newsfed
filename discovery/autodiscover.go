package discovery

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// DiscoveredFeed holds the result of a successful feed autodiscovery.
type DiscoveredFeed struct {
	FeedURL     string
	FeedType    string // "rss" or "atom"
	Title       string
	FoundDirect bool // true when Strategy 1 (direct parse) found the feed
}

// DiscoverFeed runs the three-strategy probe sequence defined in Spec 10
// section 3. Returns a DiscoveredFeed on success, or a descriptive error
// listing every URL tried and why it failed.
func DiscoverFeed(inputURL string) (*DiscoveredFeed, error) {
	type attempt struct {
		url    string
		reason string
	}
	var tried []attempt
	triedSet := map[string]bool{}

	// tryFeed attempts to parse u as a feed. On success it returns a
	// DiscoveredFeed. If u has been tried before it returns nil without
	// logging. On failure it appends to tried and returns nil.
	tryFeed := func(u string) *DiscoveredFeed {
		if triedSet[u] {
			return nil
		}
		triedSet[u] = true
		feed, err := FetchFeed(u)
		if err != nil {
			tried = append(tried, attempt{u, describeErr(err)})
			return nil
		}
		return &DiscoveredFeed{FeedURL: u, FeedType: feed.FeedType, Title: feed.Title}
	}

	// Strategy 1 -- direct parse
	if result := tryFeed(inputURL); result != nil {
		result.FoundDirect = true
		return result, nil
	}

	// Strategy 2 -- HTML link tags. The HTML-fetch outcome is recorded in
	// tried separately from the feed-parse attempts made against each
	// discovered link URL.
	if doc, err := FetchHTML(inputURL); err == nil {
		var linkURLs []string
		doc.Find(`link[rel="alternate"]`).Each(func(_ int, s *goquery.Selection) {
			t := s.AttrOr("type", "")
			if t != "application/rss+xml" && t != "application/atom+xml" {
				return
			}
			href := s.AttrOr("href", "")
			if href == "" {
				return
			}
			if resolved := resolveRef(inputURL, href); resolved != "" {
				linkURLs = append(linkURLs, resolved)
			}
		})
		for _, lu := range linkURLs {
			if result := tryFeed(lu); result != nil {
				return result, nil
			}
		}
		// Record that the HTML was fetched but yielded no usable feed links.
		if len(linkURLs) == 0 {
			tried = append(tried, attempt{inputURL, "no feed links in page"})
		}
	}

	// Strategy 3 -- common path probing. tryFeed skips any URL already in
	// triedSet, providing cross-strategy deduplication at no extra cost.
	for _, pu := range generateProbeURLs(inputURL) {
		if result := tryFeed(pu); result != nil {
			return result, nil
		}
	}

	// Build the error message. No trailing newline -- callers control
	// spacing.
	var sb strings.Builder
	fmt.Fprintf(&sb, "no feed found at %s\n\nTried:\n", inputURL)
	for _, a := range tried {
		fmt.Fprintf(&sb, "  %s -- %s\n", a.url, a.reason)
	}
	return nil, errors.New(strings.TrimRight(sb.String(), "\n"))
}

// generateProbeURLs returns candidate feed URLs for the given input URL,
// first path-relative then root-relative, skipping duplicates within the
// list. Cross-strategy deduplication is handled by the caller via tryFeed's
// triedSet.
func generateProbeURLs(inputURL string) []string {
	u, err := url.Parse(inputURL)
	if err != nil {
		return nil
	}

	// seen prevents duplicate entries within this list. inputURL is
	// pre-seeded so it can never appear as a probe candidate (it was tried in
	// Strategy 1).
	seen := map[string]bool{inputURL: true}
	var candidates []string

	add := func(rawURL string) {
		if seen[rawURL] {
			return
		}
		seen[rawURL] = true
		candidates = append(candidates, rawURL)
	}

	scheme := u.Scheme
	host := u.Host
	path := strings.TrimRight(u.Path, "/")

	// Path-relative candidates (only when path is not the root). TrimRight
	// reduces "/" to "", so the empty check handles both http://host/ and
	// http://host (no trailing slash).
	if path != "" {
		for _, suffix := range []string{"/index.xml", "/feed", "/feed.xml", "/rss.xml", "/atom.xml"} {
			add(fmt.Sprintf("%s://%s%s%s", scheme, host, path, suffix))
		}
	}

	// Root-relative candidates
	for _, p := range []string{"/index.xml", "/feed", "/feed.xml", "/rss", "/rss.xml", "/atom.xml", "/feeds/posts/default"} {
		add(fmt.Sprintf("%s://%s%s", scheme, host, p))
	}

	return candidates
}

// resolveRef resolves href relative to base, returning the absolute URL or ""
// on error.
func resolveRef(base, href string) string {
	b, err := url.Parse(base)
	if err != nil {
		return ""
	}
	h, err := url.Parse(href)
	if err != nil {
		return ""
	}
	return b.ResolveReference(h).String()
}

// httpErrRe matches gofeed's "http error: NNN" error format, anchored to
// avoid false positives from port numbers or other digit sequences.
var httpErrRe = regexp.MustCompile(`http error: (\d{3})`)

// describeErr produces a short human-readable reason for a FetchFeed failure.
func describeErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if m := httpErrRe.FindStringSubmatch(msg); m != nil {
		return m[1]
	}
	if strings.Contains(msg, "connection refused") {
		return "connection refused"
	}
	if strings.Contains(msg, "no such host") {
		return "no such host"
	}
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded") {
		return "timeout"
	}
	return "not a feed"
}
