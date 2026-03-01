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
type browserOpenCmd struct{ url string }

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
			return filtered[i].PublishedAt.After(filtered[j].PublishedAt)
		})
		return itemsLoadedMsg{items: filtered}
	}
}

func fetchSourceCmd(discSvc *discovery.DiscoveryService, src sources.Source) tea.Cmd {
	return func() tea.Msg {
		id := src.SourceID
		result, err := discSvc.SyncSources(context.Background(), &id)
		if err != nil {
			return fetchDoneMsg{err: err}
		}
		return fetchDoneMsg{itemsAdded: result.ItemsDiscovered}
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
		return m, m.loadItemsForCurrent()

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
	}

	// Global keys (no modal open)
	switch msg.String() {
	case "q":
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
	case "a":
		if m.focus == focusSources {
			return m.handleOpenSourceAdd()
		}
	}

	return m, nil
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
	for i := range m.addInputs {
		m.addInputs[i].SetValue("")
	}
	m.addFocus = 0
	m.addInputs[0].Focus()
	m.addInputs[1].Blur()
	m.addInputs[2].Blur()
	m.modal = modalSourceAdd
	return m, nil
}

func (m Model) handleSourceAddKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
		return m, nil
	case "tab":
		m.addFocus = (m.addFocus + 1) % 3
		for i := range m.addInputs {
			if i == m.addFocus {
				m.addInputs[i].Focus()
			} else {
				m.addInputs[i].Blur()
			}
		}
		return m, nil
	case "enter":
		name := m.addInputs[0].Value()
		url := m.addInputs[1].Value()
		srcType := m.addInputs[2].Value()
		if name == "" || url == "" || srcType == "" {
			m.statusMsg = "Name, URL, and type are required"
			return m, nil
		}
		now := time.Now().UTC()
		src, err := m.sourceStore.CreateSource(srcType, url, name, nil, &now)
		if err != nil {
			// Keep the modal open so the user can correct the input.
			m.statusMsg = fmt.Sprintf("Add error: %v", err)
			return m, nil
		}
		m.modal = modalNone
		return m, loadSourcesAndRestoreCursorCmd(m.sourceStore, src.SourceID)
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
