package newsfed

import (
	"fmt"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	ext "github.com/mmcdole/gofeed/extensions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFeedItemToNewsItem_BasicRSSItem verifies conversion of basic RSS item
func TestFeedItemToNewsItem_BasicRSSItem(t *testing.T) {
	publishedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	item := &gofeed.Item{
		Title:           "Test Article",
		Description:     "This is a test description",
		Link:            "http://example.com/article",
		PublishedParsed: &publishedTime,
	}

	feedTitle := "Example Feed"
	newsItem := FeedItemToNewsItem(item, feedTitle)

	assert.Equal(t, "Test Article", newsItem.Title)
	assert.Equal(t, "This is a test description", newsItem.Summary)
	assert.Equal(t, "http://example.com/article", newsItem.URL)
	require.NotNil(t, newsItem.Publisher)
	assert.Equal(t, feedTitle, *newsItem.Publisher)
	assert.Equal(t, publishedTime, newsItem.PublishedAt)
	assert.Nil(t, newsItem.PinnedAt)
}

// TestFeedItemToNewsItem_EmptyTitle verifies fallback for empty title
func TestFeedItemToNewsItem_EmptyTitle(t *testing.T) {
	item := &gofeed.Item{
		Title: "",
		Link:  "http://example.com/article",
	}

	newsItem := FeedItemToNewsItem(item, "Feed")

	assert.Equal(t, "(No title)", newsItem.Title, "should use fallback for empty title")
}

// TestFeedItemToNewsItem_NoPublisher verifies nil publisher handling
func TestFeedItemToNewsItem_NoPublisher(t *testing.T) {
	item := &gofeed.Item{
		Title: "Test",
		Link:  "http://example.com",
	}

	newsItem := FeedItemToNewsItem(item, "")

	assert.Nil(t, newsItem.Publisher, "empty feed title should result in nil publisher")
}

// TestFeedItemToNewsItem_SingleAuthor verifies single author extraction
func TestFeedItemToNewsItem_SingleAuthor(t *testing.T) {
	item := &gofeed.Item{
		Title: "Test",
		Link:  "http://example.com",
		Author: &gofeed.Person{
			Name: "John Doe",
		},
	}

	newsItem := FeedItemToNewsItem(item, "Feed")

	require.Len(t, newsItem.Authors, 1)
	assert.Equal(t, "John Doe", newsItem.Authors[0])
}

// TestFeedItemToNewsItem_MultipleAuthors verifies multiple author extraction
func TestFeedItemToNewsItem_MultipleAuthors(t *testing.T) {
	item := &gofeed.Item{
		Title: "Test",
		Link:  "http://example.com",
		Authors: []*gofeed.Person{
			{Name: "John Doe"},
			{Name: "Jane Smith"},
		},
	}

	newsItem := FeedItemToNewsItem(item, "Feed")

	assert.Len(t, newsItem.Authors, 2)
	assert.Contains(t, newsItem.Authors, "John Doe")
	assert.Contains(t, newsItem.Authors, "Jane Smith")
}

// TestFeedItemToNewsItem_AuthorDeduplication verifies no duplicate authors
func TestFeedItemToNewsItem_AuthorDeduplication(t *testing.T) {
	item := &gofeed.Item{
		Title: "Test",
		Link:  "http://example.com",
		Author: &gofeed.Person{
			Name: "John Doe",
		},
		Authors: []*gofeed.Person{
			{Name: "John Doe"}, // Duplicate
			{Name: "Jane Smith"},
		},
	}

	newsItem := FeedItemToNewsItem(item, "Feed")

	assert.Len(t, newsItem.Authors, 2, "should deduplicate authors")
	assert.Contains(t, newsItem.Authors, "John Doe")
	assert.Contains(t, newsItem.Authors, "Jane Smith")
}

// TestFeedItemToNewsItem_DublinCoreCreator verifies Dublin Core extension
func TestFeedItemToNewsItem_DublinCoreCreator(t *testing.T) {
	item := &gofeed.Item{
		Title: "Test",
		Link:  "http://example.com",
		DublinCoreExt: &ext.DublinCoreExtension{
			Creator: []string{"DC Author 1", "DC Author 2"},
		},
	}

	newsItem := FeedItemToNewsItem(item, "Feed")

	assert.Len(t, newsItem.Authors, 2)
	assert.Contains(t, newsItem.Authors, "DC Author 1")
	assert.Contains(t, newsItem.Authors, "DC Author 2")
}

// TestFeedItemToNewsItem_EmptyAuthors verifies empty author handling
func TestFeedItemToNewsItem_EmptyAuthors(t *testing.T) {
	item := &gofeed.Item{
		Title: "Test",
		Link:  "http://example.com",
		Authors: []*gofeed.Person{
			{Name: ""}, // Empty name
		},
	}

	newsItem := FeedItemToNewsItem(item, "Feed")

	assert.Empty(t, newsItem.Authors, "should skip empty author names")
}

// TestFeedItemToNewsItem_PublishedDate verifies published date handling
func TestFeedItemToNewsItem_PublishedDate(t *testing.T) {
	publishedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	item := &gofeed.Item{
		Title:           "Test",
		Link:            "http://example.com",
		PublishedParsed: &publishedTime,
	}

	newsItem := FeedItemToNewsItem(item, "Feed")

	assert.Equal(t, publishedTime, newsItem.PublishedAt)
}

// TestFeedItemToNewsItem_UpdatedDatePreferred verifies updated date takes
// priority
func TestFeedItemToNewsItem_UpdatedDatePreferred(t *testing.T) {
	publishedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	updatedTime := time.Date(2024, 1, 16, 12, 0, 0, 0, time.UTC)

	item := &gofeed.Item{
		Title:           "Test",
		Link:            "http://example.com",
		PublishedParsed: &publishedTime,
		UpdatedParsed:   &updatedTime,
	}

	newsItem := FeedItemToNewsItem(item, "Feed")

	assert.Equal(t, updatedTime, newsItem.PublishedAt, "should prefer updated date over published date")
}

// TestFeedItemToNewsItem_PublishedDateFallback verifies fallback to published
// date
func TestFeedItemToNewsItem_PublishedDateFallback(t *testing.T) {
	publishedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	item := &gofeed.Item{
		Title:           "Test",
		Link:            "http://example.com",
		PublishedParsed: &publishedTime,
	}

	newsItem := FeedItemToNewsItem(item, "Feed")

	assert.Equal(t, publishedTime, newsItem.PublishedAt, "should use published date if updated is missing")
}

// TestFeedItemToNewsItem_NoDateFallback verifies current time fallback
func TestFeedItemToNewsItem_NoDateFallback(t *testing.T) {
	before := time.Now()

	item := &gofeed.Item{
		Title: "Test",
		Link:  "http://example.com",
	}

	newsItem := FeedItemToNewsItem(item, "Feed")
	after := time.Now()

	assert.True(t, newsItem.PublishedAt.After(before) || newsItem.PublishedAt.Equal(before))
	assert.True(t, newsItem.PublishedAt.Before(after) || newsItem.PublishedAt.Equal(after))
}

// TestFeedItemToNewsItem_DiscoveredAtSet verifies discovered_at is current
// time
func TestFeedItemToNewsItem_DiscoveredAtSet(t *testing.T) {
	before := time.Now()

	item := &gofeed.Item{
		Title: "Test",
		Link:  "http://example.com",
	}

	newsItem := FeedItemToNewsItem(item, "Feed")
	after := time.Now()

	assert.True(t, newsItem.DiscoveredAt.After(before) || newsItem.DiscoveredAt.Equal(before))
	assert.True(t, newsItem.DiscoveredAt.Before(after) || newsItem.DiscoveredAt.Equal(after))
}

// TestFeedItemToNewsItem_PinnedAtNil verifies pinned_at is nil on creation
func TestFeedItemToNewsItem_PinnedAtNil(t *testing.T) {
	item := &gofeed.Item{
		Title: "Test",
		Link:  "http://example.com",
	}

	newsItem := FeedItemToNewsItem(item, "Feed")

	assert.Nil(t, newsItem.PinnedAt, "newly created items should not be pinned")
}

// TestFeedItemToNewsItem_GeneratesUUID verifies UUID is generated
func TestFeedItemToNewsItem_GeneratesUUID(t *testing.T) {
	item := &gofeed.Item{
		Title: "Test",
		Link:  "http://example.com",
	}

	newsItem1 := FeedItemToNewsItem(item, "Feed")
	newsItem2 := FeedItemToNewsItem(item, "Feed")

	assert.NotEqual(t, newsItem1.ID, newsItem2.ID, "should generate unique UUIDs")
}

// TestFeedToNewsItems_EmptyFeed verifies empty feed handling
func TestFeedToNewsItems_EmptyFeed(t *testing.T) {
	feed := &gofeed.Feed{
		Title: "Test Feed",
		Items: []*gofeed.Item{},
	}

	items := FeedToNewsItems(feed)

	assert.Empty(t, items, "should return empty slice for empty feed")
}

// TestFeedToNewsItems_SingleItem verifies single item conversion
func TestFeedToNewsItems_SingleItem(t *testing.T) {
	feed := &gofeed.Feed{
		Title: "Test Feed",
		Items: []*gofeed.Item{
			{
				Title: "Article 1",
				Link:  "http://example.com/1",
			},
		},
	}

	items := FeedToNewsItems(feed)

	require.Len(t, items, 1)
	assert.Equal(t, "Article 1", items[0].Title)
	assert.Equal(t, "Test Feed", *items[0].Publisher)
}

// TestFeedToNewsItems_MultipleItems verifies multiple item conversion
func TestFeedToNewsItems_MultipleItems(t *testing.T) {
	// Create items with different published dates
	pub1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	pub2 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	pub3 := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)

	feed := &gofeed.Feed{
		Title: "Test Feed",
		Items: []*gofeed.Item{
			{Title: "Article 1", Link: "http://example.com/1", PublishedParsed: &pub1},
			{Title: "Article 2", Link: "http://example.com/2", PublishedParsed: &pub2},
			{Title: "Article 3", Link: "http://example.com/3", PublishedParsed: &pub3},
		},
	}

	items := FeedToNewsItems(feed)

	require.Len(t, items, 3)
	// Items should be sorted by published_at (most recent first) per RFC 2
	// section 2.2.3
	assert.Equal(t, "Article 3", items[0].Title)
	assert.Equal(t, "Article 2", items[1].Title)
	assert.Equal(t, "Article 1", items[2].Title)
}

// TestFeedToNewsItems_SortsByPublishedDate verifies items are sorted by
// published_at (most recent first) per RFC 2 section 2.2.3
func TestFeedToNewsItems_SortsByPublishedDate(t *testing.T) {
	// Create items with dates in non-chronological order
	pubOld := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	pubMid := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	pubNew := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	feed := &gofeed.Feed{
		Title: "Test Feed",
		Items: []*gofeed.Item{
			{Title: "Middle", Link: "http://example.com/2", PublishedParsed: &pubMid},
			{Title: "Newest", Link: "http://example.com/3", PublishedParsed: &pubNew},
			{Title: "Oldest", Link: "http://example.com/1", PublishedParsed: &pubOld},
		},
	}

	items := FeedToNewsItems(feed)

	require.Len(t, items, 3)
	assert.Equal(t, "Newest", items[0].Title, "most recent should be first")
	assert.Equal(t, "Middle", items[1].Title)
	assert.Equal(t, "Oldest", items[2].Title, "oldest should be last")
}

// TestContains_Found verifies string is found
func TestContains_Found(t *testing.T) {
	slice := []string{"apple", "banana", "cherry"}

	assert.True(t, contains(slice, "banana"))
}

// TestContains_NotFound verifies string is not found
func TestContains_NotFound(t *testing.T) {
	slice := []string{"apple", "banana", "cherry"}

	assert.False(t, contains(slice, "orange"))
}

// TestContains_CaseInsensitive verifies case-insensitive matching
func TestContains_CaseInsensitive(t *testing.T) {
	slice := []string{"Apple", "Banana", "Cherry"}

	assert.True(t, contains(slice, "banana"), "should match case-insensitively")
	assert.True(t, contains(slice, "APPLE"), "should match case-insensitively")
	assert.True(t, contains(slice, "ChErRy"), "should match case-insensitively")
}

// TestContains_EmptySlice verifies empty slice handling
func TestContains_EmptySlice(t *testing.T) {
	slice := []string{}

	assert.False(t, contains(slice, "anything"))
}

// TestContains_EmptyString verifies empty string handling
func TestContains_EmptyString(t *testing.T) {
	slice := []string{"apple", "", "cherry"}

	assert.True(t, contains(slice, ""), "should find empty string")
}

// Property test: FeedItemToNewsItem always generates valid NewsItem
func TestFeedItemToNewsItem_AlwaysValid(t *testing.T) {
	testCases := []struct {
		name string
		item *gofeed.Item
	}{
		{
			name: "minimal item",
			item: &gofeed.Item{
				Link: "http://example.com",
			},
		},
		{
			name: "complete item",
			item: &gofeed.Item{
				Title:       "Full Article",
				Description: "Full description",
				Link:        "http://example.com/article",
				Author:      &gofeed.Person{Name: "Author"},
				PublishedParsed: func() *time.Time {
					t := time.Now()
					return &t
				}(),
			},
		},
		{
			name: "item with all fields empty",
			item: &gofeed.Item{
				Title:       "",
				Description: "",
				Link:        "",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			newsItem := FeedItemToNewsItem(tc.item, "Feed")

			// Verify required fields are present
			assert.NotEqual(t, "", newsItem.Title, "title should never be empty")
			assert.NotEmpty(t, newsItem.ID, "ID should be generated")
			assert.NotZero(t, newsItem.PublishedAt, "published_at should be set")
			assert.NotZero(t, newsItem.DiscoveredAt, "discovered_at should be set")
			assert.NotNil(t, newsItem.Authors, "authors should be initialized")
		})
	}
}

// Property test: FeedToNewsItems caps at 20 items per RFC 2 section 2.2.3
func TestFeedToNewsItems_CapsAt20Items(t *testing.T) {
	testCases := []struct {
		inputCount    int
		expectedCount int
	}{
		{0, 0},
		{1, 1},
		{5, 5},
		{10, 10},
		{20, 20},
		{25, 20},  // Capped at 20
		{100, 20}, // Capped at 20
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("input_%d", tc.inputCount), func(t *testing.T) {
			feed := &gofeed.Feed{
				Title: "Test",
				Items: make([]*gofeed.Item, tc.inputCount),
			}

			for i := range tc.inputCount {
				feed.Items[i] = &gofeed.Item{
					Title: "Article",
					Link:  "http://example.com",
				}
			}

			items := FeedToNewsItems(feed)

			assert.Len(t, items, tc.expectedCount,
				"output length should be min(input, 20)")
		})
	}
}

// TestFeedToNewsItems_SelectsMostRecent20 verifies that when a feed has more
// than 20 items, the 20 most recent (by published_at) are selected per RFC 2
// section 2.2.3
func TestFeedToNewsItems_SelectsMostRecent20(t *testing.T) {
	// Create 30 items with ascending dates
	feed := &gofeed.Feed{
		Title: "Test Feed",
		Items: make([]*gofeed.Item, 30),
	}

	baseDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range 30 {
		pubDate := baseDate.Add(time.Duration(i) * 24 * time.Hour)
		feed.Items[i] = &gofeed.Item{
			Title:           fmt.Sprintf("Article %d", i),
			Link:            fmt.Sprintf("http://example.com/%d", i),
			PublishedParsed: &pubDate,
		}
	}

	items := FeedToNewsItems(feed)

	// Should return exactly 20 items
	require.Len(t, items, 20)

	// Should return the 20 most recent (items 10-29) First item should be the
	// newest (item 29)
	assert.Equal(t, "Article 29", items[0].Title,
		"first item should be the newest")
	// Last item should be the 20th newest (item 10)
	assert.Equal(t, "Article 10", items[19].Title,
		"last item should be the 20th newest")

	// Verify all items are in descending order
	for i := 0; i < len(items)-1; i++ {
		assert.True(t, items[i].PublishedAt.After(items[i+1].PublishedAt),
			"items should be in descending order by published_at")
	}
}

// Property test: contains is reflexive for valid strings
func TestContains_Reflexive(t *testing.T) {
	testStrings := []string{"apple", "Banana", "CHERRY", "123", "test-value"}

	for _, str := range testStrings {
		slice := []string{str}
		assert.True(t, contains(slice, str), "slice containing string should find it")
	}
}
