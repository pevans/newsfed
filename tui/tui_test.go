package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/pevans/newsfed/newsfeed"
	"github.com/pevans/newsfed/sources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -- Helpers --

func newModel() Model {
	return Model{
		width:      80,
		height:     24,
		editInputs: [2]textinput.Model{textinput.New(), textinput.New()},
		addInputs:  [2]textinput.Model{textinput.New(), textinput.New()},
	}
}

func makeSource(name, url, typ string, enabled bool) sources.Source {
	var enabledAt *time.Time
	if enabled {
		t := time.Now()
		enabledAt = &t
	}
	return sources.Source{
		SourceID:   uuid.New(),
		Name:       name,
		URL:        url,
		SourceType: typ,
		EnabledAt:  enabledAt,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

func makeSourceWithFetch(name string, fetchedAt *time.Time) sources.Source {
	s := makeSource(name, "https://example.com/feed", "rss", true)
	s.LastFetchedAt = fetchedAt
	return s
}

func makeItem(title, publisher string, published time.Time) newsfeed.NewsItem {
	return newsfeed.NewsItem{
		ID:          uuid.New(),
		Title:       title,
		URL:         "https://example.com/" + title,
		Publisher:   &publisher,
		PublishedAt: published,
	}
}

func pressKey(m Model, key string) (Model, tea.Cmd) {
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return result.(Model), cmd
}

func pressSpecialKey(m Model, key tea.KeyType) (Model, tea.Cmd) {
	result, cmd := m.Update(tea.KeyMsg{Type: key})
	return result.(Model), cmd
}

// -- formatRelativeLabel --

func TestFormatRelativeLabel_nilReturnsNever(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, "(never)", formatRelativeLabel(nil, now))
}

func TestFormatRelativeLabel_todayReturnsToday(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-time.Minute)
	assert.Equal(t, "(today)", formatRelativeLabel(&recent, now))
}

func TestFormatRelativeLabel_pastReturnsAgo(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	past := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, "(1y ago)", formatRelativeLabel(&past, now))
}

// -- relativeDate --

func TestRelativeDate_futureOrNowReturnsToday(t *testing.T) {
	now := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, "today", relativeDate(now.Add(time.Hour), now), "future should be today")
	assert.Equal(t, "today", relativeDate(now, now), "equal time should be today")
}

func TestRelativeDate_earlierTodayReturnsToday(t *testing.T) {
	now := time.Date(2026, 2, 28, 20, 0, 0, 0, time.UTC)
	earlier := time.Date(2026, 2, 28, 6, 0, 0, 0, time.UTC)
	assert.Equal(t, "today", relativeDate(earlier, now))
}

func TestRelativeDate_daysOnly(t *testing.T) {
	now := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)
	for days := 1; days <= 5; days++ {
		pub := now.AddDate(0, 0, -days)
		assert.Equal(t, fmt.Sprintf("%dd", days), relativeDate(pub, now),
			"exactly %d day(s) ago", days)
	}
}

func TestRelativeDate_exactMonthsOmitsDays(t *testing.T) {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	pub := now.AddDate(0, -2, 0) // exactly 2 months before
	assert.Equal(t, "2mo", relativeDate(pub, now))
}

func TestRelativeDate_monthsAndDays(t *testing.T) {
	now := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)
	pub := time.Date(2026, 1, 26, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, "1mo2d", relativeDate(pub, now))
}

func TestRelativeDate_yearsMonthsDays(t *testing.T) {
	now := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)
	pub := time.Date(2022, 12, 27, 0, 0, 0, 0, time.UTC)
	// Dec 27 + 3y = 2025-12-27, +2mo = 2026-02-27, +1d = 2026-02-28
	assert.Equal(t, "3y2mo1d", relativeDate(pub, now))
}

func TestRelativeDate_exactYearOmitsMonthsAndDays(t *testing.T) {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	pub := now.AddDate(-1, 0, 0) // exactly 1 year before
	assert.Equal(t, "1y", relativeDate(pub, now))
}

func TestRelativeDate_neverEmpty(t *testing.T) {
	now := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)
	// Any past date should produce a non-empty string.
	for daysBack := 1; daysBack <= 365*3; daysBack++ {
		pub := now.AddDate(0, 0, -daysBack)
		assert.NotEmpty(t, relativeDate(pub, now),
			"daysBack=%d should produce a non-empty result", daysBack)
	}
}

// -- wordWrap / wrapParagraph --

func TestWrapParagraph_allWordsPresent(t *testing.T) {
	text := "the quick brown fox jumps over the lazy dog"
	got := wrapParagraph(text, 20)
	// Every word from the original must appear in the output.
	for _, word := range strings.Fields(text) {
		assert.Contains(t, got, word)
	}
}

func TestWrapParagraph_noLineExceedsWidth(t *testing.T) {
	text := "the quick brown fox jumps over the lazy dog"
	widths := []int{5, 10, 15, 40, 80}
	for _, w := range widths {
		got := wrapParagraph(text, w)
		for _, line := range strings.Split(got, "\n") {
			assert.LessOrEqual(t, utf8.RuneCountInString(line), w,
				"width=%d: line %q exceeded limit", w, line)
		}
	}
}

func TestWrapParagraph_emptyStringReturnsEmpty(t *testing.T) {
	assert.Equal(t, "", wrapParagraph("", 80))
}

func TestWordWrap_preservesParagraphBreaks(t *testing.T) {
	text := "first paragraph\n\nsecond paragraph\n\nthird paragraph"
	got := wordWrap(text, 80)
	assert.Contains(t, got, "\n\n", "paragraph breaks should be preserved")
	parts := strings.Split(got, "\n\n")
	require.Len(t, parts, 3)
	assert.Contains(t, parts[0], "first")
	assert.Contains(t, parts[1], "second")
	assert.Contains(t, parts[2], "third")
}

func TestWordWrap_zeroWidthReturnsInput(t *testing.T) {
	text := "some text"
	assert.Equal(t, text, wordWrap(text, 0))
}

// -- stripHTML --

func TestStripHTML_plainTextUnchanged(t *testing.T) {
	text := "just plain text"
	assert.Equal(t, text, stripHTML(text))
}

func TestStripHTML_removesTagsPreservesText(t *testing.T) {
	cases := []struct {
		html string
		want string
	}{
		{"<p>hello</p>", "hello"},
		{"<b>bold</b> and <i>italic</i>", "bold and italic"},
		{"<div><p>nested</p></div>", "nested"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, stripHTML(c.html), "input: %q", c.html)
	}
}

func TestStripHTML_trimsWhitespace(t *testing.T) {
	got := stripHTML("  <p>  text  </p>  ")
	assert.Equal(t, "text", got)
}

// -- renderSourceList --

func TestRenderSourceList_neverFetchedShowsNever(t *testing.T) {
	m := newModel()
	m.sources = []sources.Source{makeSourceWithFetch("Example", nil)}
	got := m.renderSourceList(40, 5)
	assert.Contains(t, got, "(never)")
	assert.NotContains(t, got, "(today)")
}

func TestRenderSourceList_recentFetchShowsToday(t *testing.T) {
	m := newModel()
	now := time.Now()
	m.sources = []sources.Source{makeSourceWithFetch("Example", &now)}
	got := m.renderSourceList(40, 5)
	assert.Contains(t, got, "(today)")
}

func TestRenderSourceList_pastFetchShowsRelativeAgo(t *testing.T) {
	m := newModel()
	past := time.Now().AddDate(-1, 0, 0)
	m.sources = []sources.Source{makeSourceWithFetch("Example", &past)}
	got := m.renderSourceList(40, 5)
	assert.Contains(t, got, " ago)")
	assert.NotContains(t, got, "(today)")
	assert.NotContains(t, got, "(never)")
}

func TestRenderSourceList_longNameTruncated(t *testing.T) {
	m := newModel()
	// width=20: prefixLen=3, date="(never)"=7, minSpace=1 → nameMax=9
	// "VeryLongName" (12 chars) must be truncated to 9 chars with "..."
	m.sources = []sources.Source{makeSourceWithFetch("VeryLongName", nil)}
	got := m.renderSourceList(20, 5)
	assert.Contains(t, got, "...")
	assert.NotContains(t, got, "VeryLongName")
}

func TestRenderSourceList_nameExactlyFitsNoTruncation(t *testing.T) {
	m := newModel()
	// width=25: prefixLen=3, date="(never)"=7, minSpace=1 → nameMax=14
	name := strings.Repeat("x", 14)
	m.sources = []sources.Source{makeSourceWithFetch(name, nil)}
	got := m.renderSourceList(25, 5)
	assert.Contains(t, got, name)
	assert.NotContains(t, got, "...")
}

func TestRenderSourceList_extremelyNarrowDoesNotPanic(t *testing.T) {
	m := newModel()
	m.sources = []sources.Source{makeSourceWithFetch("Name", nil)}
	// Width 5 forces nameMaxWidth to be clamped to 1; must not panic.
	got := m.renderSourceList(5, 5)
	assert.NotEmpty(t, got)
}

// -- Cursor movement --

func TestMoveCursorDown_sourcesFocusWraps(t *testing.T) {
	m := newModel()
	m.focus = focusSources
	m.sources = []sources.Source{makeSource("A", "u1", "rss", true), makeSource("B", "u2", "rss", true)}
	m.sourceCursor = 1

	m = m.moveCursorDown()
	assert.Equal(t, 0, m.sourceCursor, "cursor should wrap to 0 after last item")
}

func TestMoveCursorUp_sourcesFocusWraps(t *testing.T) {
	m := newModel()
	m.focus = focusSources
	m.sources = []sources.Source{makeSource("A", "u1", "rss", true), makeSource("B", "u2", "rss", true)}
	m.sourceCursor = 0

	m = m.moveCursorUp()
	assert.Equal(t, 1, m.sourceCursor, "cursor should wrap to last item from first")
}

func TestMoveCursorDown_itemsFocusWraps(t *testing.T) {
	m := newModel()
	m.focus = focusItems
	now := time.Now()
	m.items = []newsfeed.NewsItem{makeItem("A", "src", now), makeItem("B", "src", now)}
	m.itemCursor = 1

	m = m.moveCursorDown()
	assert.Equal(t, 0, m.itemCursor)
}

func TestMoveCursorUp_itemsFocusWraps(t *testing.T) {
	m := newModel()
	m.focus = focusItems
	now := time.Now()
	m.items = []newsfeed.NewsItem{makeItem("A", "src", now), makeItem("B", "src", now)}
	m.itemCursor = 0

	m = m.moveCursorUp()
	assert.Equal(t, 1, m.itemCursor)
}

func TestMoveCursor_emptyListNoOp(t *testing.T) {
	m := newModel()
	m.focus = focusSources
	m.sources = nil

	before := m.sourceCursor
	m = m.moveCursorDown()
	assert.Equal(t, before, m.sourceCursor)
	m = m.moveCursorUp()
	assert.Equal(t, before, m.sourceCursor)
}

func TestMoveCursorDown_advancesNormally(t *testing.T) {
	m := newModel()
	m.focus = focusSources
	m.sources = []sources.Source{
		makeSource("A", "u1", "rss", true),
		makeSource("B", "u2", "rss", true),
		makeSource("C", "u3", "rss", true),
	}
	m.sourceCursor = 0

	m = m.moveCursorDown()
	assert.Equal(t, 1, m.sourceCursor)
	m = m.moveCursorDown()
	assert.Equal(t, 2, m.sourceCursor)
}

// -- Update: message handling --

func TestUpdate_windowSizeMsg(t *testing.T) {
	m := newModel()
	result, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	got := result.(Model)
	assert.Equal(t, 120, got.width)
	assert.Equal(t, 40, got.height)
	assert.Nil(t, cmd)
}

func TestUpdate_sourcesLoadedMsg_preservesOrder(t *testing.T) {
	// Update does not re-sort; sorting is done by loadSourcesCmd before the
	// message is dispatched. Verify that Update stores items in the order
	// given.
	m := newModel()
	msg := sourcesLoadedMsg{
		items: []sources.Source{
			makeSource("alpha", "u1", "rss", true),
			makeSource("Beta", "u2", "rss", true),
			makeSource("Zeal", "u3", "rss", true),
		},
	}
	result, _ := m.Update(msg)
	got := result.(Model)
	require.Len(t, got.sources, 3)
	assert.Equal(t, "alpha", got.sources[0].Name)
	assert.Equal(t, "Beta", got.sources[1].Name)
	assert.Equal(t, "Zeal", got.sources[2].Name)
}

func TestUpdate_sourcesLoadedMsg_clampsCursor(t *testing.T) {
	m := newModel()
	m.sourceCursor = 5
	msg := sourcesLoadedMsg{items: []sources.Source{makeSource("A", "u1", "rss", true)}}
	result, _ := m.Update(msg)
	got := result.(Model)
	assert.Equal(t, 0, got.sourceCursor)
}

func TestUpdate_sourcesLoadedMsg_restoresIDCursor(t *testing.T) {
	m := newModel()
	srcA := makeSource("alpha", "u1", "rss", true)
	srcB := makeSource("beta", "u2", "rss", true)
	srcC := makeSource("gamma", "u3", "rss", true)
	msg := sourcesLoadedMsg{
		items:     []sources.Source{srcA, srcB, srcC},
		restoreID: &srcB.SourceID,
	}
	result, _ := m.Update(msg)
	got := result.(Model)
	assert.Equal(t, 1, got.sourceCursor, "cursor should point to beta (index 1)")
}

func TestUpdate_sourcesLoadedMsg_restoreFallsBackToZeroWhenIDMissing(t *testing.T) {
	m := newModel()
	missingID := uuid.New()
	msg := sourcesLoadedMsg{
		items:     []sources.Source{makeSource("A", "u1", "rss", true)},
		restoreID: &missingID,
	}
	result, _ := m.Update(msg)
	got := result.(Model)
	assert.Equal(t, 0, got.sourceCursor)
}

func TestUpdate_sourcesLoadedMsg_errorSetsStatusMsg(t *testing.T) {
	m := newModel()
	msg := sourcesLoadedMsg{err: fmt.Errorf("db unavailable")}
	result, cmd := m.Update(msg)
	got := result.(Model)
	assert.Contains(t, got.statusMsg, "db unavailable")
	assert.Nil(t, cmd)
	assert.Nil(t, got.sources)
}

func TestUpdate_itemsLoadedMsg_setsItems(t *testing.T) {
	m := newModel()
	now := time.Now()
	items := []newsfeed.NewsItem{makeItem("A", "src", now), makeItem("B", "src", now)}
	result, cmd := m.Update(itemsLoadedMsg{items: items})
	got := result.(Model)
	assert.Len(t, got.items, 2)
	assert.Nil(t, cmd)
}

func TestUpdate_itemsLoadedMsg_errorSetsStatusMsg(t *testing.T) {
	m := newModel()
	result, _ := m.Update(itemsLoadedMsg{err: fmt.Errorf("read error")})
	got := result.(Model)
	assert.Contains(t, got.statusMsg, "read error")
}

func TestUpdate_itemsLoadedMsg_clampsCursor(t *testing.T) {
	m := newModel()
	m.itemCursor = 10
	now := time.Now()
	result, _ := m.Update(itemsLoadedMsg{items: []newsfeed.NewsItem{makeItem("A", "src", now)}})
	got := result.(Model)
	assert.Equal(t, 0, got.itemCursor)
}

func TestUpdate_fetchDoneMsg_clearsFeching(t *testing.T) {
	m := newModel()
	m.fetching = true
	m.sources = []sources.Source{makeSource("A", "u1", "rss", true)}
	result, _ := m.Update(fetchDoneMsg{itemsAdded: 3})
	got := result.(Model)
	assert.False(t, got.fetching)
	assert.Contains(t, got.statusMsg, "3")
}

func TestUpdate_fetchDoneMsg_error(t *testing.T) {
	m := newModel()
	m.fetching = true
	m.sources = []sources.Source{makeSource("A", "u1", "rss", true)}
	result, _ := m.Update(fetchDoneMsg{err: fmt.Errorf("timeout")})
	got := result.(Model)
	assert.False(t, got.fetching)
	assert.Contains(t, got.statusMsg, "timeout")
}

// -- Key handling: global (no modal) --

func TestKey_qQuitsProgram(t *testing.T) {
	m := newModel()
	_, cmd := pressKey(m, "q")
	require.NotNil(t, cmd)
	// tea.Quit returns a Msg of type tea.QuitMsg when executed.
	msg := cmd()
	assert.IsType(t, tea.QuitMsg{}, msg)
}

func TestKey_tabTogglesFocus(t *testing.T) {
	m := newModel()
	assert.Equal(t, focusSources, m.focus)

	m2, _ := pressKey(m, "tab")
	assert.Equal(t, focusItems, m2.focus)

	m3, _ := pressKey(m2, "tab")
	assert.Equal(t, focusSources, m3.focus)
}

func TestKey_tabClearsStatusMsg(t *testing.T) {
	m := newModel()
	m.statusMsg = "some message"
	m2, _ := pressKey(m, "tab")
	assert.Empty(t, m2.statusMsg)
}

func TestKey_upDownSourcesFrame_clearsStatusMsg(t *testing.T) {
	m := newModel()
	m.focus = focusSources
	m.statusMsg = "stale status"
	m.sources = []sources.Source{
		makeSource("A", "u1", "rss", true),
		makeSource("B", "u2", "rss", true),
	}

	m2, _ := pressKey(m, "j")
	assert.Empty(t, m2.statusMsg)

	m2.statusMsg = "stale status"
	m3, _ := pressKey(m2, "k")
	assert.Empty(t, m3.statusMsg)
}

func TestKey_enterSourcesFrame_opensSourceModal(t *testing.T) {
	m := newModel()
	m.focus = focusSources
	m.sources = []sources.Source{makeSource("A", "u1", "rss", true)}

	m2, _ := pressSpecialKey(m, tea.KeyEnter)
	assert.Equal(t, modalSourceManagement, m2.modal)
}

func TestKey_enterItemsFrame_opensItemDetailModal(t *testing.T) {
	m := newModel()
	m.focus = focusItems
	m.items = []newsfeed.NewsItem{makeItem("A", "src", time.Now())}

	m2, _ := pressSpecialKey(m, tea.KeyEnter)
	assert.Equal(t, modalItemDetail, m2.modal)
}

func TestKey_enterEmptySourcesList_noModal(t *testing.T) {
	m := newModel()
	m.focus = focusSources
	m.sources = nil

	m2, _ := pressSpecialKey(m, tea.KeyEnter)
	assert.Equal(t, modalNone, m2.modal)
}

func TestKey_rDisabledSource_setsStatusMsg(t *testing.T) {
	m := newModel()
	m.sources = []sources.Source{makeSource("A", "u1", "rss", false)}

	m2, cmd := pressKey(m, "r")
	assert.Contains(t, m2.statusMsg, "disabled")
	assert.Nil(t, cmd)
	assert.False(t, m2.fetching)
}

func TestKey_rEnabledSource_startsFetch(t *testing.T) {
	m := newModel()
	// discSvc is nil but we only check that fetching=true and cmd is
	// returned; the cmd itself is not executed in this unit test.
	m.sources = []sources.Source{makeSource("A", "u1", "rss", true)}

	m2, cmd := pressKey(m, "r")
	assert.True(t, m2.fetching)
	assert.Equal(t, "Fetching...", m2.statusMsg)
	assert.NotNil(t, cmd)
}

func TestKey_rAlreadyFetching_noOp(t *testing.T) {
	m := newModel()
	m.fetching = true
	m.statusMsg = "Fetching..."
	m.sources = []sources.Source{makeSource("A", "u1", "rss", true)}

	m2, cmd := pressKey(m, "r")
	assert.True(t, m2.fetching)
	assert.Nil(t, cmd)
}

// -- Key handling: source management modal --

func TestSourceManagementModal_escCloses(t *testing.T) {
	m := newModel()
	m.modal = modalSourceManagement
	m.sources = []sources.Source{makeSource("A", "u1", "rss", true)}

	m2, _ := pressSpecialKey(m, tea.KeyEsc)
	assert.Equal(t, modalNone, m2.modal)
}

func TestSourceManagementModal_jkMoveCursor(t *testing.T) {
	m := newModel()
	m.modal = modalSourceManagement
	m.sources = []sources.Source{makeSource("A", "u1", "rss", true)}
	m.sourceModalCursor = 0

	m2, _ := pressKey(m, "j")
	assert.Equal(t, 1, m2.sourceModalCursor)

	m3, _ := pressKey(m2, "k")
	assert.Equal(t, 0, m3.sourceModalCursor)
}

func TestSourceManagementModal_enterOnEditOpensEditForm(t *testing.T) {
	m := newModel()
	m.modal = modalSourceManagement
	m.sourceModalCursor = 0 // Edit
	src := makeSource("MySrc", "https://feed.example.com", "rss", true)
	m.sources = []sources.Source{src}

	m2, _ := pressSpecialKey(m, tea.KeyEnter)
	assert.Equal(t, modalSourceEdit, m2.modal)
	assert.Equal(t, "MySrc", m2.editInputs[0].Value())
	assert.Equal(t, "https://feed.example.com", m2.editInputs[1].Value())
}

func TestSourceManagementModal_enterOnDeleteOpensDeleteConfirm(t *testing.T) {
	m := newModel()
	m.modal = modalSourceManagement
	m.sourceModalCursor = 1 // Delete
	m.sources = []sources.Source{makeSource("A", "u1", "rss", true)}

	m2, _ := pressSpecialKey(m, tea.KeyEnter)
	assert.Equal(t, modalSourceDeleteConfirm, m2.modal)
	assert.Equal(t, 1, m2.deleteConfirmCursor, "should default to No")
}

// -- Key handling: delete confirm modal --

func TestDeleteConfirmModal_escCloses(t *testing.T) {
	m := newModel()
	m.modal = modalSourceDeleteConfirm
	m.sources = []sources.Source{makeSource("A", "u1", "rss", true)}

	m2, _ := pressSpecialKey(m, tea.KeyEsc)
	assert.Equal(t, modalNone, m2.modal)
}

func TestDeleteConfirmModal_leftRightMoveCursor(t *testing.T) {
	m := newModel()
	m.modal = modalSourceDeleteConfirm
	m.deleteConfirmCursor = 1 // No
	m.sources = []sources.Source{makeSource("A", "u1", "rss", true)}

	m2, _ := pressSpecialKey(m, tea.KeyLeft)
	assert.Equal(t, 0, m2.deleteConfirmCursor, "left should select Yes")

	m3, _ := pressSpecialKey(m, tea.KeyRight)
	assert.Equal(t, 1, m3.deleteConfirmCursor, "right should select No")
}

func TestDeleteConfirmModal_enterNoClosesModal(t *testing.T) {
	m := newModel()
	m.modal = modalSourceDeleteConfirm
	m.deleteConfirmCursor = 1 // No
	m.sources = []sources.Source{makeSource("A", "u1", "rss", true)}

	m2, _ := pressSpecialKey(m, tea.KeyEnter)
	assert.Equal(t, modalNone, m2.modal)
}

// -- Key handling: item detail modal --

func TestItemDetailModal_escCloses(t *testing.T) {
	m := newModel()
	m.modal = modalItemDetail
	m.items = []newsfeed.NewsItem{makeItem("A", "src", time.Now())}

	m2, _ := pressSpecialKey(m, tea.KeyEsc)
	assert.Equal(t, modalNone, m2.modal)
}

func TestItemDetailModal_oReturnsCmd(t *testing.T) {
	m := newModel()
	m.modal = modalItemDetail
	m.items = []newsfeed.NewsItem{makeItem("A", "src", time.Now())}

	_, cmd := pressKey(m, "o")
	assert.NotNil(t, cmd, "pressing o should return a browser-open command")
}

func TestItemDetailModal_oOnEmptyItemsNoCmd(t *testing.T) {
	m := newModel()
	m.modal = modalItemDetail
	m.items = nil

	_, cmd := pressKey(m, "o")
	assert.Nil(t, cmd)
}

// -- Edit form: tab and esc --

func TestEditModal_tabSwitchesField(t *testing.T) {
	m := newModel()
	m.modal = modalSourceEdit
	m.editFocus = 0
	m.sources = []sources.Source{makeSource("A", "u1", "rss", true)}

	m2, _ := pressSpecialKey(m, tea.KeyTab)
	assert.Equal(t, 1, m2.editFocus)

	m3, _ := pressSpecialKey(m2, tea.KeyTab)
	assert.Equal(t, 0, m3.editFocus)
}

func TestEditModal_escClosesModal(t *testing.T) {
	m := newModel()
	m.modal = modalSourceEdit
	m.sources = []sources.Source{makeSource("A", "u1", "rss", true)}

	m2, _ := pressSpecialKey(m, tea.KeyEsc)
	assert.Equal(t, modalNone, m2.modal)
}

// -- renderModeLine --

func TestRenderModeLine_defaultShowsHelp(t *testing.T) {
	m := newModel()
	m.statusMsg = ""
	got := m.renderModeLine()
	assert.Contains(t, got, "[Q]uit")
	assert.Contains(t, got, "[R]efresh")
	assert.Contains(t, got, "[Tab]")
	assert.Contains(t, got, "[Enter]")
}

func TestRenderModeLine_statusMsgReplaceHelp(t *testing.T) {
	m := newModel()
	m.statusMsg = "Fetching..."
	got := m.renderModeLine()
	assert.Contains(t, got, "Fetching...")
	assert.NotContains(t, got, "[Q]uit")
}

func TestRenderModeLine_clearedStatusRestoresHelp(t *testing.T) {
	m := newModel()
	m.statusMsg = "Fetching..."
	m.statusMsg = "" // cleared
	got := m.renderModeLine()
	assert.Contains(t, got, "[Q]uit")
}

// -- renderSourceFields --

func TestRenderSourceFields_includesAllFields(t *testing.T) {
	last := time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC)
	errMsg := "connection refused"
	interval := "2h"
	src := sources.Source{
		SourceID:        uuid.New(),
		Name:            "Test Source",
		URL:             "https://example.com/feed",
		SourceType:      "rss",
		CreatedAt:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:       time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		LastFetchedAt:   &last,
		PollingInterval: &interval,
		FetchErrorCount: 3,
		LastError:       &errMsg,
	}

	got := renderSourceFields(src)
	assert.Contains(t, got, "Test Source")
	assert.Contains(t, got, "https://example.com/feed")
	assert.Contains(t, got, "rss")
	assert.Contains(t, got, "2024-01-01")
	assert.Contains(t, got, "2024-02-01")
	assert.Contains(t, got, "2024-05-10")
	assert.Contains(t, got, "2h")
	assert.Contains(t, got, "3")
	assert.Contains(t, got, "connection refused")
}

func TestRenderSourceFields_neverFetchedShowsNever(t *testing.T) {
	src := makeSource("A", "u", "rss", true)
	src.LastFetchedAt = nil
	got := renderSourceFields(src)
	assert.Contains(t, got, "Never")
}

// -- renderModeLine: [A]dd source hint --

func TestRenderModeLine_sourcesFrameIncludesAddHint(t *testing.T) {
	m := newModel()
	m.focus = focusSources
	got := m.renderModeLine()
	assert.Contains(t, got, "[A]dd source")
}

func TestRenderModeLine_itemsFrameOmitsAddHint(t *testing.T) {
	m := newModel()
	m.focus = focusItems
	got := m.renderModeLine()
	assert.NotContains(t, got, "[A]dd source")
}

func TestRenderModeLine_statusMsgSuppressesAddHint(t *testing.T) {
	m := newModel()
	m.focus = focusSources
	m.statusMsg = "Fetching..."
	got := m.renderModeLine()
	assert.NotContains(t, got, "[A]dd source")
}

// -- Key handling: 'a' --

func TestKey_aSourcesFocus_opensAddModal(t *testing.T) {
	m := newModel()
	m.focus = focusSources

	m2, _ := pressKey(m, "a")
	assert.Equal(t, modalSourceAdd, m2.modal)
	assert.Equal(t, 0, m2.addFocus)
}

func TestKey_aSourcesFocus_clearsInputs(t *testing.T) {
	m := newModel()
	m.focus = focusSources
	m.addInputs[0].SetValue("leftover")
	m.addInputs[1].SetValue("leftover")

	m2, _ := pressKey(m, "a")
	assert.Empty(t, m2.addInputs[0].Value())
	assert.Empty(t, m2.addInputs[1].Value())
}

func TestKey_aItemsFocus_noEffect(t *testing.T) {
	m := newModel()
	m.focus = focusItems

	m2, _ := pressKey(m, "a")
	assert.Equal(t, modalNone, m2.modal)
}

// -- Key handling: add source modal --

func TestAddModal_escClosesModal(t *testing.T) {
	m := newModel()
	m.modal = modalSourceAdd

	m2, _ := pressSpecialKey(m, tea.KeyEsc)
	assert.Equal(t, modalNone, m2.modal)
}

func TestAddModal_tabCyclesTwoFields(t *testing.T) {
	m := newModel()
	m.modal = modalSourceAdd
	m.addFocus = 0

	m2, _ := pressSpecialKey(m, tea.KeyTab)
	assert.Equal(t, 1, m2.addFocus)

	m3, _ := pressSpecialKey(m2, tea.KeyTab)
	assert.Equal(t, 0, m3.addFocus)
}

func TestAddModal_enterWithAllEmptyKeepsModalOpen(t *testing.T) {
	m := newModel()
	m.modal = modalSourceAdd

	m2, _ := pressSpecialKey(m, tea.KeyEnter)
	assert.Equal(t, modalSourceAdd, m2.modal)
	assert.NotEmpty(t, m2.statusMsg)
}

func TestAddModal_enterWithEmptyURLKeepsModalOpen(t *testing.T) {
	m := newModel()
	m.modal = modalSourceAdd
	m.addInputs[0].SetValue("My Source")
	// URL is still empty.

	m2, _ := pressSpecialKey(m, tea.KeyEnter)
	assert.Equal(t, modalSourceAdd, m2.modal)
	assert.NotEmpty(t, m2.statusMsg)
}

// -- renderSourceAddModal --

func TestRenderSourceAddModal_containsLabels(t *testing.T) {
	m := newModel()
	got := m.renderSourceAddModal()
	assert.Contains(t, got, "Add Source")
	assert.Contains(t, got, "Name:")
	assert.Contains(t, got, "URL:")
}

func TestRenderSourceAddModal_showsDiscoveringWhenActive(t *testing.T) {
	m := newModel()
	m.addDiscovering = true
	got := m.renderSourceAddModal()
	assert.Contains(t, got, "Discovering feed...")
}

func TestRenderSourceAddModal_showsStatusMsgOnFailure(t *testing.T) {
	m := newModel()
	m.addDiscovering = false
	m.statusMsg = "No feed found. Check the URL and try again."
	got := m.renderSourceAddModal()
	assert.Contains(t, got, "No feed found.")
}

func TestRenderSourceAddModal_statusMsgHiddenWhileDiscovering(t *testing.T) {
	// During discovery, "Discovering feed..." takes precedence over any
	// status message that may be set.
	m := newModel()
	m.addDiscovering = true
	m.statusMsg = "stale message"
	got := m.renderSourceAddModal()
	assert.Contains(t, got, "Discovering feed...")
	assert.NotContains(t, got, "stale message")
}

func TestRenderSourceFields_datesAreYYYYMMDD(t *testing.T) {
	fixed := time.Date(2024, 6, 15, 12, 34, 56, 0, time.UTC)
	src := makeSource("A", "u", "rss", true)
	src.CreatedAt = fixed
	src.UpdatedAt = fixed
	src.LastFetchedAt = &fixed

	got := renderSourceFields(src)

	// The date "2024-06-15" should appear; "12:34:56" must not.
	assert.Contains(t, got, "2024-06-15", "date value should appear in YYYY-MM-DD format")
	assert.NotContains(t, got, "12:34:56", "time-of-day component should not appear in date fields")
}
