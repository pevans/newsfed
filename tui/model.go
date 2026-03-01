package tui

import (
	"io"
	"log"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pevans/newsfed/discovery"
	"github.com/pevans/newsfed/newsfeed"
	"github.com/pevans/newsfed/sources"
)

type focusArea int

const (
	focusSources focusArea = iota
	focusItems
)

type modalKind int

const (
	modalNone modalKind = iota
	modalSourceManagement
	modalSourceEdit
	modalSourceDeleteConfirm
	modalItemDetail
)

// Model is the Bubble Tea model for the TUI.
type Model struct {
	width, height int
	focus         focusArea
	sources       []sources.Source
	items         []newsfeed.NewsItem
	sourceCursor  int
	itemCursor    int
	modal         modalKind

	// Source management modal
	sourceModalCursor   int // 0=Edit, 1=Delete
	deleteConfirmCursor int // 0=Yes, 1=No (No is default)

	// Edit form
	editInputs [2]textinput.Model // [0]=Name, [1]=URL
	editFocus  int

	// Item detail modal
	itemDetailScroll int

	// Status
	statusMsg string
	fetching  bool

	// Storage handles
	sourceStore *sources.SourceStore
	newsFeed    *newsfeed.NewsFeed
	discSvc     *discovery.DiscoveryService
}

// Run opens the terminal, creates the Bubble Tea program, and blocks until
// quit.
func Run(sourceStore *sources.SourceStore, newsFeed *newsfeed.NewsFeed, discSvc *discovery.DiscoveryService) error {
	nameInput := textinput.New()
	nameInput.Placeholder = "Source name"
	nameInput.Focus()

	urlInput := textinput.New()
	urlInput.Placeholder = "Feed URL"

	m := Model{
		sourceStore: sourceStore,
		newsFeed:    newsFeed,
		discSvc:     discSvc,
		editInputs:  [2]textinput.Model{nameInput, urlInput},
	}

	// Silence the default logger while the TUI is running. The discovery
	// service emits log.Printf lines that would corrupt the display.
	prev := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(prev)

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
