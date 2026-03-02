package discovery

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const rssBody = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test RSS Feed</title>
    <link>http://example.com</link>
    <description>A test feed</description>
  </channel>
</rss>`

func TestDiscoverFeed_DirectRSS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(rssBody))
	}))
	defer srv.Close()

	result, err := DiscoverFeed(srv.URL)
	require.NoError(t, err)
	assert.Equal(t, srv.URL, result.FeedURL)
	assert.Equal(t, "rss", result.FeedType)
	assert.Equal(t, "Test RSS Feed", result.Title)
	assert.True(t, result.FoundDirect, "direct feed parse should set FoundDirect")
}

func TestDiscoverFeed_HTMLLinkTags(t *testing.T) {
	var feedURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(rssBody))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html><html><head>
<link rel="alternate" type="application/rss+xml" href="%s">
</head><body>Hello</body></html>`, feedURL)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	feedURL = srv.URL + "/feed.xml"

	result, err := DiscoverFeed(srv.URL + "/")
	require.NoError(t, err)
	assert.Equal(t, feedURL, result.FeedURL)
	assert.Equal(t, "rss", result.FeedType)
	assert.False(t, result.FoundDirect, "link-tag discovery should not set FoundDirect")
}

// TestDiscoverFeed_ProbeIndexXML_RootPath exercises the root-relative probe
// candidate `/index.xml` when the input URL path is "/".
func TestDiscoverFeed_ProbeIndexXML_RootPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/index.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(rssBody))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head></head><body>Hello</body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	result, err := DiscoverFeed(srv.URL + "/")
	require.NoError(t, err)
	assert.Equal(t, srv.URL+"/index.xml", result.FeedURL)
	assert.False(t, result.FoundDirect)
}

// TestDiscoverFeed_ProbeIndexXML_SubPath exercises the path-relative probe
// candidate `{path}/index.xml` when the input URL has a non-root path.
func TestDiscoverFeed_ProbeIndexXML_SubPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/blog/index.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(rssBody))
	})
	mux.HandleFunc("/blog/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head></head><body>Hello</body></html>`))
	})
	// All other paths return 404.
	srv := httptest.NewServer(mux)
	defer srv.Close()

	result, err := DiscoverFeed(srv.URL + "/blog/")
	require.NoError(t, err)
	assert.Equal(t, srv.URL+"/blog/index.xml", result.FeedURL)
	assert.False(t, result.FoundDirect)
}

func TestDiscoverFeed_NoFeed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Not Found"))
	}))
	defer srv.Close()

	inputURL := srv.URL + "/"
	_, err := DiscoverFeed(inputURL)
	require.Error(t, err)

	msg := err.Error()
	assert.Contains(t, msg, "no feed found at "+inputURL)
	assert.Contains(t, msg, "Tried:")
	// The input URL itself must appear in the tried list.
	assert.Contains(t, msg, inputURL)
	// At least one reason should be present (404 from the server).
	assert.Contains(t, msg, "404")
}

func TestDiscoverFeed_HTMLLinkTagsNoLinks_LogsNoFeedLinksInPage(t *testing.T) {
	// When the page fetches as HTML but has no qualifying link tags, the
	// error output should record "no feed links in page" for the input URL.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head></head><body>Hello</body></html>`))
	}))
	defer srv.Close()

	inputURL := srv.URL + "/"
	_, err := DiscoverFeed(inputURL)
	require.Error(t, err)

	msg := err.Error()
	assert.Contains(t, msg, "no feed links in page")
}

func TestDiscoverFeed_RelativeHrefResolved(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(rssBody))
	})
	mux.HandleFunc("/post/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		// Use a relative href
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head>
<link rel="alternate" type="application/rss+xml" href="/feed.xml">
</head><body>Hello</body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	result, err := DiscoverFeed(srv.URL + "/post/")
	require.NoError(t, err)
	assert.Equal(t, srv.URL+"/feed.xml", result.FeedURL)
}

func TestGenerateProbeURLs_NoDuplicates(t *testing.T) {
	// When path is "/", path-relative candidates would duplicate
	// root-relative ones -- none should appear.
	urls := generateProbeURLs("http://example.com/")
	seen := map[string]bool{}
	for _, u := range urls {
		assert.False(t, seen[u], "duplicate probe URL: %s", u)
		seen[u] = true
	}
	// Root-relative candidates should still be present.
	assert.True(t, seen["http://example.com/index.xml"])
	assert.True(t, seen["http://example.com/feed"])
}

func TestGenerateProbeURLs_PathRelativeFirst(t *testing.T) {
	urls := generateProbeURLs("http://example.com/blog/")
	// The first candidate should be path-relative.
	assert.True(t, strings.HasPrefix(urls[0], "http://example.com/blog/"))
}

func TestDiscoverFeed_CrossStrategyDedup(t *testing.T) {
	// If Strategy 2 discovers a link URL that is also a Strategy 3 probe
	// candidate, it should only be tried once.
	var tries int
	mux := http.NewServeMux()
	mux.HandleFunc("/index.xml", func(w http.ResponseWriter, r *http.Request) {
		tries++
		// Return non-feed content so discovery fails, to exercise the full
		// path.
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("not a feed"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		// Link tag points to /index.xml, which is also a Strategy 3 probe.
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head>
<link rel="alternate" type="application/rss+xml" href="/index.xml">
</head><body>Hello</body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, _ = DiscoverFeed(srv.URL + "/")
	assert.Equal(t, 1, tries, "/index.xml should only be fetched once across all strategies")
}
