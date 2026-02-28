package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/pevans/newsfed/sources"
)

var (
	focusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62"))

	blurredBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240"))

	// selectedStyle inverts video for the highlighted row. Must be applied
	// before applying lipgloss styles, since selectedStyle adds ANSI codes.
	selectedStyle = lipgloss.NewStyle().Reverse(true)

	modalBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(1, 2)
)

// modalBorderOverhead is the total horizontal space consumed by the modal's
// border (1 char each side) and padding (2 chars each side).
const modalBorderOverhead = 6

// View renders the full TUI.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	mainView := m.renderMain()

	if m.modal != modalNone {
		return m.renderWithModal(mainView)
	}

	return mainView
}

func (m Model) renderMain() string {
	// Each frame gets roughly half the terminal width, accounting for
	// borders. Each border takes 2 chars (left + right), so inner width =
	// (total - 4) / 2.
	totalInner := m.width - 4
	if totalInner < 4 {
		totalInner = 4
	}
	frameInner := totalInner / 2

	// Frame height minus a small margin; inner height subtracts top+bottom
	// border.
	frameHeight := m.height - 2
	if frameHeight < 4 {
		frameHeight = 4
	}
	innerHeight := frameHeight - 2
	if innerHeight < 1 {
		innerHeight = 1
	}

	leftContent := m.renderSourceList(frameInner, innerHeight)
	rightContent := m.renderItemList(frameInner, innerHeight)

	var leftStyle, rightStyle lipgloss.Style
	if m.focus == focusSources {
		leftStyle = focusedBorderStyle
		rightStyle = blurredBorderStyle
	} else {
		leftStyle = blurredBorderStyle
		rightStyle = focusedBorderStyle
	}

	leftFrame := leftStyle.Width(frameInner).Height(innerHeight).Render(leftContent)
	rightFrame := rightStyle.Width(frameInner).Height(innerHeight).Render(rightContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftFrame, rightFrame)
}

func (m Model) renderSourceList(width, height int) string {
	if len(m.sources) == 0 {
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Render("No sources.")
	}

	// Reserve the last line for the status message so it is always visible.
	listHeight := height
	if m.statusMsg != "" {
		listHeight = height - 1
	}

	// Each source occupies 2 lines. Determine which source the viewport
	// starts from so the cursor stays visible.
	linesPerSource := 2
	visibleSources := listHeight / linesPerSource
	if visibleSources < 1 {
		visibleSources = 1
	}

	startSource := 0
	if m.sourceCursor >= visibleSources {
		startSource = m.sourceCursor - visibleSources + 1
	}

	var lines []string
	for i := startSource; i < len(m.sources); i++ {
		if len(lines) >= listHeight {
			break
		}
		src := m.sources[i]
		line1 := ansi.Truncate(fmt.Sprintf("%d. %s (%s)", i+1, src.Name, src.SourceType), width, "...")
		line2 := ansi.Truncate(fmt.Sprintf("Last updated: %s", formatDate(src.LastFetchedAt)), width, "...")

		if i == m.sourceCursor {
			line1 = selectedStyle.Width(width).Render(line1)
			line2 = selectedStyle.Width(width).Render(line2)
		}

		lines = append(lines, line1, line2)
	}

	if m.statusMsg != "" {
		// Pad with blank lines to push status to the bottom of the frame.
		for len(lines) < listHeight {
			lines = append(lines, "")
		}
		lines = append(lines, ansi.Truncate(m.statusMsg, width, "..."))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderItemList(width, height int) string {
	if len(m.items) == 0 {
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Render("<No items>")
	}

	// Each item occupies 3 lines. Determine how many items fit on screen and
	// which item the viewport starts from so the cursor stays visible.
	linesPerItem := 3
	visibleItems := height / linesPerItem
	if visibleItems < 1 {
		visibleItems = 1
	}

	startItem := 0
	if m.itemCursor >= visibleItems {
		startItem = m.itemCursor - visibleItems + 1
	}

	var lines []string
	for i := startItem; i < len(m.items); i++ {
		if len(lines) >= height {
			break
		}
		item := m.items[i]
		line1 := ansi.Truncate(fmt.Sprintf("%d. %s", i+1, item.Title), width, "...")
		line2 := ansi.Truncate(fmt.Sprintf("Authors: %s", strings.Join(item.Authors, ", ")), width, "...")
		line3 := ansi.Truncate(fmt.Sprintf("Published: %s", item.PublishedAt.Format("2006-01-02")), width, "...")

		if i == m.itemCursor {
			line1 = selectedStyle.Width(width).Render(line1)
			line2 = selectedStyle.Width(width).Render(line2)
			line3 = selectedStyle.Width(width).Render(line3)
		}

		lines = append(lines, line1, line2, line3)
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderWithModal(background string) string {
	modalContent := ""
	switch m.modal {
	case modalSourceManagement:
		modalContent = m.renderSourceManagementModal()
	case modalSourceEdit:
		modalContent = m.renderSourceEditModal()
	case modalSourceDeleteConfirm:
		modalContent = m.renderDeleteConfirmModal()
	case modalItemDetail:
		modalContent = m.renderItemDetailModal()
	}

	modal := modalBorderStyle.Render(modalContent)

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		modal,
		lipgloss.WithWhitespaceChars(" "),
	)
}

func (m Model) renderSourceManagementModal() string {
	if len(m.sources) == 0 {
		return ""
	}
	src := m.sources[m.sourceCursor]

	var sb strings.Builder
	sb.WriteString(renderSourceFields(src))
	sb.WriteString("\n")

	editLabel := "Edit"
	deleteLabel := "Delete"

	if m.sourceModalCursor == 0 {
		editLabel = selectedStyle.Render(editLabel)
	} else {
		deleteLabel = selectedStyle.Render(deleteLabel)
	}

	sb.WriteString(editLabel + "\n")
	sb.WriteString(deleteLabel)

	return sb.String()
}

func renderSourceFields(src sources.Source) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Name:             %s\n", src.Name))
	sb.WriteString(fmt.Sprintf("URL:              %s\n", src.URL))
	sb.WriteString(fmt.Sprintf("Type:             %s\n", src.SourceType))
	sb.WriteString(fmt.Sprintf("Enabled:          %v\n", src.IsEnabled()))
	sb.WriteString(fmt.Sprintf("Created At:       %s\n", src.CreatedAt.Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("Updated At:       %s\n", src.UpdatedAt.Format("2006-01-02")))
	if src.LastFetchedAt != nil {
		sb.WriteString(fmt.Sprintf("Last Fetched At:  %s\n", src.LastFetchedAt.Format("2006-01-02")))
	} else {
		sb.WriteString("Last Fetched At:  Never\n")
	}
	if src.PollingInterval != nil {
		sb.WriteString(fmt.Sprintf("Polling Interval: %s\n", *src.PollingInterval))
	} else {
		sb.WriteString("Polling Interval: (default)\n")
	}
	sb.WriteString(fmt.Sprintf("Error Count:      %d\n", src.FetchErrorCount))
	if src.LastError != nil {
		sb.WriteString(fmt.Sprintf("Last Error:       %s\n", *src.LastError))
	}
	return sb.String()
}

func (m Model) renderSourceEditModal() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Name: %s\n", m.editInputs[0].View()))
	sb.WriteString(fmt.Sprintf("URL:  %s", m.editInputs[1].View()))
	return sb.String()
}

func (m Model) renderDeleteConfirmModal() string {
	if len(m.sources) == 0 {
		return ""
	}
	src := m.sources[m.sourceCursor]

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Delete %q? This cannot be undone.\n\n", src.Name))

	yesLabel := "[ Yes ]"
	noLabel := "[ No ]"

	if m.deleteConfirmCursor == 0 {
		yesLabel = selectedStyle.Render(yesLabel)
	} else {
		noLabel = selectedStyle.Render(noLabel)
	}

	sb.WriteString(yesLabel + "   " + noLabel)
	return sb.String()
}

func (m Model) renderItemDetailModal() string {
	if len(m.items) == 0 {
		return ""
	}
	item := m.items[m.itemCursor]

	// The modal's text width accounts for the border and padding on each
	// side.
	modalWidth := m.width*60/100 - modalBorderOverhead
	if modalWidth < 40 {
		modalWidth = 40
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Title:     %s\n", item.Title))
	sb.WriteString(fmt.Sprintf("Published: %s\n", item.PublishedAt.Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("URL:       %s\n", item.URL))

	if item.Summary != "" {
		plain := stripHTML(item.Summary)
		if plain != "" {
			sb.WriteString("\n")
			sb.WriteString(wordWrap(plain, modalWidth))
		}
	}

	// Apply scroll offset. The modal border and padding consume 4 lines
	// vertically (1 border + 1 padding on each side), so the visible height
	// is the terminal height minus that overhead.
	lines := strings.Split(sb.String(), "\n")
	// Drop a trailing empty element produced by a final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	visibleHeight := m.height - 4
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	scroll := m.itemDetailScroll
	maxScroll := len(lines) - visibleHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}

	end := scroll + visibleHeight
	if end > len(lines) {
		end = len(lines)
	}

	return strings.Join(lines[scroll:end], "\n")
}

// stripHTML removes HTML tags from a string using goquery.
func stripHTML(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return html
	}
	return strings.TrimSpace(doc.Text())
}

// wordWrap wraps text to the given width, preserving paragraph breaks (blank
// lines). Each paragraph is wrapped independently.
func wordWrap(text string, width int) string {
	if width <= 0 {
		return text
	}

	paragraphs := strings.Split(text, "\n\n")
	wrapped := make([]string, 0, len(paragraphs))
	for _, para := range paragraphs {
		wrapped = append(wrapped, wrapParagraph(strings.TrimSpace(para), width))
	}
	return strings.Join(wrapped, "\n\n")
}

func wrapParagraph(text string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	var lines []string
	current := words[0]

	for _, word := range words[1:] {
		if utf8.RuneCountInString(current)+1+utf8.RuneCountInString(word) <= width {
			current += " " + word
		} else {
			lines = append(lines, current)
			current = word
		}
	}
	lines = append(lines, current)

	return strings.Join(lines, "\n")
}

// formatDate renders a *time.Time as YYYY-MM-DD, or "Never" if nil.
func formatDate(t *time.Time) string {
	if t == nil {
		return "Never"
	}
	return t.Format("2006-01-02")
}
