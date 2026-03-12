package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/pevans/newsfed/discovery"
	"github.com/pevans/newsfed/newsfeed"
	"github.com/pevans/newsfed/sources"
)

type sourceDiscoveredMsg struct {
	feedURL    string
	feedType   string
	feedName   string
	err        error
	generation int
}

type refreshAllStartedMsg struct {
	ch    <-chan discovery.SourceProgress
	errCh <-chan error
}

type refreshAllSyncErrMsg struct{ err error }

type refreshAllProgressMsg struct {
	progress discovery.SourceProgress
	ch       <-chan discovery.SourceProgress
}

type refreshAllDoneMsg struct{}

type sourceCreatedMsg struct {
	src *sources.Source
	err error
}

type itemPinToggledMsg struct {
	err error
}

// togglePinCmd toggles the pinned state of the given item and persists the
// change to storage.
func togglePinCmd(feed *newsfeed.NewsFeed, item newsfeed.NewsItem) tea.Cmd {
	return func() tea.Msg {
		if item.PinnedAt == nil {
			now := time.Now().UTC()
			item.PinnedAt = &now
		} else {
			item.PinnedAt = nil
		}
		return itemPinToggledMsg{err: feed.Update(item)}
	}
}

func discoverAndAddSourceCmd(name, inputURL string, generation int) tea.Cmd {
	return func() tea.Msg {
		// Per Spec 10 section 5.2
		ctx, cancel := context.WithTimeout(context.Background(), discovery.AutodiscoverTimeout)
		defer cancel()
		result, err := discovery.DiscoverFeed(ctx, inputURL)
		if err != nil {
			return sourceDiscoveredMsg{err: err, generation: generation}
		}
		feedName := name
		if feedName == "" {
			feedName = result.Title
		}
		return sourceDiscoveredMsg{
			feedURL:    result.FeedURL,
			feedType:   result.FeedType,
			feedName:   feedName,
			generation: generation,
		}
	}
}

func createSourceCmd(store *sources.SourceStore, feedType, feedURL, feedName string) tea.Cmd {
	return func() tea.Msg {
		now := time.Now().UTC()
		src, err := store.CreateSource(feedType, feedURL, feedName, nil, &now)
		return sourceCreatedMsg{src: src, err: err}
	}
}

// -- Message types --

type sourcesLoadedMsg struct {
	items     []sources.Source
	err       error
	restoreID *uuid.UUID // if non-nil, reposition cursor to this source after reload
}
type itemsLoadedMsg struct {
	items []newsfeed.NewsItem
	err   error
}
type fetchDoneMsg struct {
	itemsAdded int
	err        error
}

// Init loads sources on startup.
func (m Model) Init() tea.Cmd {
	return loadSourcesCmd(m.sourceStore)
}

func loadSourcesCmd(store *sources.SourceStore) tea.Cmd {
	return func() tea.Msg {
		list, err := store.ListSources(sources.SourceFilter{})
		if err != nil {
			return sourcesLoadedMsg{err: err}
		}
		sort.Slice(list, func(i, j int) bool {
			return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
		})
		return sourcesLoadedMsg{items: list}
	}
}

// loadSourcesAndRestoreCursorCmd reloads sources and repositions the cursor
// to the source with the given ID, if it still exists.
func loadSourcesAndRestoreCursorCmd(store *sources.SourceStore, restoreID uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		list, err := store.ListSources(sources.SourceFilter{})
		if err != nil {
			return sourcesLoadedMsg{err: err}
		}
		sort.Slice(list, func(i, j int) bool {
			return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
		})
		return sourcesLoadedMsg{items: list, restoreID: &restoreID}
	}
}

func loadItemsCmd(feed *newsfeed.NewsFeed, sourceID uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		result, err := feed.List()
		if err != nil {
			return itemsLoadedMsg{err: err}
		}
		var filtered []newsfeed.NewsItem
		for _, item := range result.Items {
			if item.SourceID != nil && *item.SourceID == sourceID {
				filtered = append(filtered, item)
			}
		}
		sort.Slice(filtered, func(i, j int) bool {
			iPinned := filtered[i].PinnedAt != nil
			jPinned := filtered[j].PinnedAt != nil
			if iPinned != jPinned {
				return iPinned
			}
			return filtered[i].PublishedAt.After(filtered[j].PublishedAt)
		})
		return itemsLoadedMsg{items: filtered}
	}
}

func fetchSourceCmd(discSvc *discovery.DiscoveryService, src sources.Source) tea.Cmd {
	return func() tea.Msg {
		id := src.SourceID
		result, err := discSvc.SyncSources(context.Background(), &id, nil)
		if err != nil {
			return fetchDoneMsg{err: err}
		}
		return fetchDoneMsg{itemsAdded: result.ItemsDiscovered}
	}
}

// startRefreshAllCmd starts a concurrent sync of all enabled sources, routing
// per-source progress updates through a buffered channel. The channel is
// closed by SyncSources after all fetches complete. ctx allows the caller to
// cancel the sync (e.g. on quit).
func startRefreshAllCmd(ctx context.Context, discSvc *discovery.DiscoveryService, bufSize int) tea.Cmd {
	return func() tea.Msg {
		progressCh := make(chan discovery.SourceProgress, bufSize)
		errCh := make(chan error, 1)
		go func() {
			_, err := discSvc.SyncSources(ctx, nil, progressCh)
			if err != nil {
				errCh <- err
			}
			close(errCh)
		}()
		return refreshAllStartedMsg{ch: progressCh, errCh: errCh}
	}
}

// listenRefreshAllCmd blocks until the next progress update arrives on ch,
// then returns it as a refreshAllProgressMsg. When ch is closed it returns
// refreshAllDoneMsg to signal completion.
func listenRefreshAllCmd(ch <-chan discovery.SourceProgress) tea.Cmd {
	return func() tea.Msg {
		progress, ok := <-ch
		if !ok {
			return refreshAllDoneMsg{}
		}
		return refreshAllProgressMsg{progress: progress, ch: ch}
	}
}

// listenRefreshAllErrCmd blocks until the error channel from SyncSources is
// closed. If SyncSources returned an error, it is forwarded as
// refreshAllSyncErrMsg; otherwise nil is returned (no message dispatched).
func listenRefreshAllErrCmd(errCh <-chan error) tea.Cmd {
	return func() tea.Msg {
		err, ok := <-errCh
		if !ok || err == nil {
			return nil
		}
		return refreshAllSyncErrMsg{err: err}
	}
}

// openBrowserCmd launches the system browser in a goroutine so the event loop
// is not blocked.
func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "linux":
			cmd = exec.Command("xdg-open", url)
		case "windows":
			cmd = exec.Command("cmd", "/c", "start", url)
		default:
			return nil
		}
		_ = cmd.Start()
		return nil
	}
}

// Update handles all incoming messages and key events.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case sourcesLoadedMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error loading sources: %v", msg.err)
			return m, nil
		}
		m.sources = msg.items
		// Restore cursor to a specific source ID if requested (e.g. after
		// edit).
		if msg.restoreID != nil {
			m.sourceCursor = 0
			for i, s := range m.sources {
				if s.SourceID == *msg.restoreID {
					m.sourceCursor = i
					break
				}
			}
		} else if m.sourceCursor >= len(m.sources) {
			m.sourceCursor = max(0, len(m.sources)-1)
		}
		return m, m.loadItemsForCurrent()

	case itemsLoadedMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error loading items: %v", msg.err)
			return m, nil
		}
		m.items = msg.items
		if m.itemCursor >= len(m.items) {
			m.itemCursor = max(0, len(m.items)-1)
		}
		return m, nil

	case fetchDoneMsg:
		m.fetching = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Fetch error: %v", msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("Fetched: %d new item(s)", msg.itemsAdded)
		}
		var restoreID uuid.UUID
		if m.sourceCursor < len(m.sources) {
			restoreID = m.sources[m.sourceCursor].SourceID
		}
		return m, tea.Batch(
			m.loadItemsForCurrent(),
			loadSourcesAndRestoreCursorCmd(m.sourceStore, restoreID),
		)

	case sourceDiscoveredMsg:
		// Discard results that belong to a previous discovery session.
		if msg.generation != m.addGeneration {
			return m, nil
		}
		m.addDiscovering = false
		if msg.err != nil {
			m.statusMsg = "No feed found. Check the URL and try again."
			return m, nil
		}
		return m, createSourceCmd(m.sourceStore, msg.feedType, msg.feedURL, msg.feedName)

	case sourceCreatedMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Add error: %v", msg.err)
			return m, nil
		}
		m.modal = modalNone
		return m, loadSourcesAndRestoreCursorCmd(m.sourceStore, msg.src.SourceID)

	case itemPinToggledMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Pin error: %v", msg.err)
			return m, nil
		}
		return m, m.loadItemsForCurrent()

	case refreshAllStartedMsg:
		return m, tea.Batch(listenRefreshAllCmd(msg.ch), listenRefreshAllErrCmd(msg.errCh))

	case refreshAllProgressMsg:
		if m.refreshAllProgress == nil {
			m.refreshAllProgress = make(map[uuid.UUID]discovery.SourceProgress)
		}
		m.refreshAllProgress[msg.progress.Source.SourceID] = msg.progress
		// Auto-scroll to keep the most recently updated source visible.
		for i, src := range m.refreshAllSources {
			if src.SourceID == msg.progress.Source.SourceID {
				m = m.autoScrollRefreshAll(i)
				break
			}
		}
		return m, listenRefreshAllCmd(msg.ch)

	case refreshAllDoneMsg:
		m.refreshAllDone = true
		return m, nil

	case refreshAllSyncErrMsg:
		m.refreshAllSyncErr = msg.err
		m.refreshAllDone = true
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.modal {
	case modalSourceManagement:
		return m.handleSourceManagementKey(msg)
	case modalSourceEdit:
		return m.handleSourceEditKey(msg)
	case modalSourceDeleteConfirm:
		return m.handleDeleteConfirmKey(msg)
	case modalItemDetail:
		return m.handleItemDetailKey(msg)
	case modalSourceAdd:
		return m.handleSourceAddKey(msg)
	case modalRefreshAll:
		return m.handleRefreshAllKey(msg)
	}

	// Global keys (no modal open)
	switch msg.String() {
	case "q":
		if m.refreshAllCancel != nil {
			m.refreshAllCancel()
		}
		return m, tea.Quit
	case "tab":
		if m.focus == focusSources {
			m.focus = focusItems
		} else {
			m.focus = focusSources
		}
		m.statusMsg = ""
		return m, nil
	case "up", "k":
		m = m.moveCursorUp()
		if m.focus == focusSources {
			m.statusMsg = ""
			return m, m.loadItemsForCurrent()
		}
		return m, nil
	case "down", "j":
		m = m.moveCursorDown()
		if m.focus == focusSources {
			m.statusMsg = ""
			return m, m.loadItemsForCurrent()
		}
		return m, nil
	case "enter":
		return m.handleEnter()
	case "r":
		return m.handleFetch()
	case "R":
		return m.handleRefreshAll()
	case "a":
		if m.focus == focusSources {
			return m.handleOpenSourceAdd()
		}
	case "P", "p":
		if m.focus == focusItems {
			return m.handleTogglePin()
		}
	}

	return m, nil
}

func (m Model) handleTogglePin() (tea.Model, tea.Cmd) {
	if len(m.items) == 0 {
		return m, nil
	}
	return m, togglePinCmd(m.newsFeed, m.items[m.itemCursor])
}

func (m Model) moveCursorUp() Model {
	if m.focus == focusSources {
		if len(m.sources) == 0 {
			return m
		}
		m.sourceCursor = (m.sourceCursor - 1 + len(m.sources)) % len(m.sources)
	} else {
		if len(m.items) == 0 {
			return m
		}
		m.itemCursor = (m.itemCursor - 1 + len(m.items)) % len(m.items)
	}
	return m
}

func (m Model) moveCursorDown() Model {
	if m.focus == focusSources {
		if len(m.sources) == 0 {
			return m
		}
		m.sourceCursor = (m.sourceCursor + 1) % len(m.sources)
	} else {
		if len(m.items) == 0 {
			return m
		}
		m.itemCursor = (m.itemCursor + 1) % len(m.items)
	}
	return m
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	if m.focus == focusSources && len(m.sources) > 0 {
		m.modal = modalSourceManagement
		m.sourceModalCursor = 0
		return m, nil
	}
	if m.focus == focusItems && len(m.items) > 0 {
		m.modal = modalItemDetail
		m.itemDetailScroll = 0
		return m, nil
	}
	return m, nil
}

func (m Model) handleFetch() (tea.Model, tea.Cmd) {
	if len(m.sources) == 0 || m.fetching {
		return m, nil
	}
	src := m.sources[m.sourceCursor]
	if !src.IsEnabled() {
		m.statusMsg = "Source is disabled"
		return m, nil
	}
	m.fetching = true
	m.statusMsg = "Fetching..."
	return m, fetchSourceCmd(m.discSvc, src)
}

func (m Model) loadItemsForCurrent() tea.Cmd {
	if len(m.sources) == 0 {
		return func() tea.Msg { return itemsLoadedMsg{} }
	}
	src := m.sources[m.sourceCursor]
	return loadItemsCmd(m.newsFeed, src.SourceID)
}

func (m Model) handleSourceManagementKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
		return m, nil
	case "up", "k":
		m.sourceModalCursor = 0
	case "down", "j":
		m.sourceModalCursor = 1
	case "enter":
		if m.sourceModalCursor == 0 {
			src := m.sources[m.sourceCursor]
			m.editInputs[0].SetValue(src.Name)
			m.editInputs[1].SetValue(src.URL)
			m.editFocus = 0
			m.editInputs[0].Focus()
			m.editInputs[1].Blur()
			m.modal = modalSourceEdit
		} else {
			m.deleteConfirmCursor = 1 // default to No
			m.modal = modalSourceDeleteConfirm
		}
	}
	return m, nil
}

func (m Model) handleSourceEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
		return m, nil
	case "tab":
		m.editFocus = 1 - m.editFocus
		if m.editFocus == 0 {
			m.editInputs[0].Focus()
			m.editInputs[1].Blur()
		} else {
			m.editInputs[1].Focus()
			m.editInputs[0].Blur()
		}
		return m, nil
	case "enter":
		src := m.sources[m.sourceCursor]
		newName := m.editInputs[0].Value()
		newURL := m.editInputs[1].Value()
		update := sources.SourceUpdate{
			Name: &newName,
			URL:  &newURL,
		}
		if err := m.sourceStore.UpdateSource(src.SourceID, update); err != nil {
			m.statusMsg = fmt.Sprintf("Update error: %v", err)
			m.modal = modalNone
			return m, nil
		}
		m.modal = modalNone
		// Reload and reposition cursor to the same source by ID.
		return m, loadSourcesAndRestoreCursorCmd(m.sourceStore, src.SourceID)
	}

	var cmd tea.Cmd
	m.editInputs[m.editFocus], cmd = m.editInputs[m.editFocus].Update(msg)
	return m, cmd
}

func (m Model) handleDeleteConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
		return m, nil
	case "left", "h":
		m.deleteConfirmCursor = 0 // Yes
	case "right", "l":
		m.deleteConfirmCursor = 1 // No
	case "enter":
		if m.deleteConfirmCursor == 1 {
			// No -- close without deleting.
			m.modal = modalNone
			return m, nil
		}
		// Yes -- delete.
		src := m.sources[m.sourceCursor]
		if err := m.sourceStore.DeleteSource(src.SourceID); err != nil {
			m.statusMsg = fmt.Sprintf("Delete error: %v", err)
			m.modal = modalNone
			return m, nil
		}
		m.modal = modalNone
		return m, loadSourcesCmd(m.sourceStore)
	}
	return m, nil
}

func (m Model) handleOpenSourceAdd() (tea.Model, tea.Cmd) {
	m.addInputs[0].SetValue("")
	m.addInputs[1].SetValue("")
	m.addFocus = 0
	m.addInputs[0].Focus()
	m.addInputs[1].Blur()
	m.addDiscovering = false
	m.statusMsg = ""
	m.modal = modalSourceAdd
	return m, nil
}

func (m Model) handleSourceAddKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.addDiscovering {
		// Ignore all keys while discovery is in progress.
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.modal = modalNone
		return m, nil
	case "tab":
		m.addFocus = (m.addFocus + 1) % 2
		for i := range m.addInputs {
			if i == m.addFocus {
				m.addInputs[i].Focus()
			} else {
				m.addInputs[i].Blur()
			}
		}
		return m, nil
	case "enter":
		url := m.addInputs[1].Value()
		if url == "" {
			m.statusMsg = "URL is required"
			return m, nil
		}
		name := m.addInputs[0].Value()
		m.addGeneration++
		m.addDiscovering = true
		m.statusMsg = "Discovering feed..."
		return m, discoverAndAddSourceCmd(name, url, m.addGeneration)
	}

	var cmd tea.Cmd
	m.addInputs[m.addFocus], cmd = m.addInputs[m.addFocus].Update(msg)
	return m, cmd
}

func (m Model) handleItemDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
	case "o":
		if len(m.items) > 0 {
			return m, openBrowserCmd(m.items[m.itemCursor].URL)
		}
	case "up", "k":
		if m.itemDetailScroll > 0 {
			m.itemDetailScroll--
		}
	case "down", "j":
		_, maxScroll := m.itemDetailLines()
		if m.itemDetailScroll < maxScroll {
			m.itemDetailScroll++
		}
	}
	return m, nil
}

// handleRefreshAll opens the Refresh All modal and starts syncing every
// enabled source concurrently. Implements Spec 11 section 2.
func (m Model) handleRefreshAll() (tea.Model, tea.Cmd) {
	if m.modal == modalRefreshAll {
		return m, nil // already in progress -- ignore per Spec 11 section 6.2
	}

	// Collect enabled sources (m.sources is already sorted alphabetically).
	var enabled []sources.Source
	for _, src := range m.sources {
		if src.IsEnabled() {
			enabled = append(enabled, src)
		}
	}

	if len(enabled) == 0 {
		m.statusMsg = "No enabled sources"
		return m, nil
	}

	m.modal = modalRefreshAll
	m.refreshAllSources = enabled
	m.refreshAllProgress = make(map[uuid.UUID]discovery.SourceProgress)
	m.refreshAllScroll = 0
	m.refreshAllDone = false
	m.refreshAllSyncErr = nil

	ctx, cancel := context.WithCancel(context.Background())
	m.refreshAllCancel = cancel

	// Buffer size: each source sends at most 2 messages (fetching + result).
	return m, startRefreshAllCmd(ctx, m.discSvc, len(enabled)*2)
}

// refreshAllSummary iterates the refresh-all progress map and returns the
// total new items and total failed-source count.
func (m Model) refreshAllSummary() (totalNew, totalFailed int) {
	for _, p := range m.refreshAllProgress {
		switch p.Status {
		case discovery.ProgressDone:
			totalNew += p.NewItems
		case discovery.ProgressError:
			totalFailed++
		}
	}
	return
}

// handleRefreshAllKey handles key events while the Refresh All modal is open.
// Implements Spec 11 section 5.4 (scrolling) and 5.5 (dismissal).
func (m Model) handleRefreshAllKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		if !m.refreshAllDone {
			// Cannot dismiss while a refresh is still in progress.
			return m, nil
		}
		// Compute dismissal summary.
		totalNew, totalFailed := m.refreshAllSummary()
		m.modal = modalNone
		if totalFailed > 0 {
			m.statusMsg = fmt.Sprintf("Refreshed all: %d new item(s), %d failed", totalNew, totalFailed)
		} else {
			m.statusMsg = fmt.Sprintf("Refreshed all: %d new item(s)", totalNew)
		}
		// Store per-source new-item counts for the sources frame to display
		// in place of relative dates (Spec 11, Section 5.5).
		m.refreshAllNewCounts = make(map[uuid.UUID]int)
		for id, p := range m.refreshAllProgress {
			if p.Status == discovery.ProgressDone {
				m.refreshAllNewCounts[id] = p.NewItems
			}
		}
		var restoreID uuid.UUID
		if m.sourceCursor < len(m.sources) {
			restoreID = m.sources[m.sourceCursor].SourceID
		}
		return m, tea.Batch(
			m.loadItemsForCurrent(),
			loadSourcesAndRestoreCursorCmd(m.sourceStore, restoreID),
		)
	case "up", "k":
		if m.refreshAllScroll > 0 {
			m.refreshAllScroll--
		}
	case "down", "j":
		maxScroll := len(m.refreshAllSources) - m.refreshAllVisibleSources()
		if maxScroll < 0 {
			maxScroll = 0
		}
		if m.refreshAllScroll < maxScroll {
			m.refreshAllScroll++
		}
	}
	return m, nil
}

// refreshAllVisibleSources returns the number of source rows that fit in the
// Refresh All modal's scrollable list area.
func (m Model) refreshAllVisibleSources() int {
	// 80 % of terminal height, minus 4 lines of border/padding overhead,
	// minus 2 lines reserved for the blank separator and summary line.
	maxHeight := m.height * 80 / 100
	visible := maxHeight - 4 - 2
	if visible < 1 {
		visible = 1
	}
	return visible
}

// autoScrollRefreshAll adjusts refreshAllScroll so that the source at
// sourceIdx is visible.
func (m Model) autoScrollRefreshAll(sourceIdx int) Model {
	visible := m.refreshAllVisibleSources()
	if sourceIdx < m.refreshAllScroll {
		m.refreshAllScroll = sourceIdx
	} else if sourceIdx >= m.refreshAllScroll+visible {
		m.refreshAllScroll = sourceIdx - visible + 1
	}
	return m
}
