package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/pevans/newsfed/discovery"
	"github.com/pevans/newsfed/sources"
)

var (
	focusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("117"))

	blurredBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240"))

	// selectedStyle inverts video for the highlighted row. Must be applied
	// before applying lipgloss styles, since selectedStyle adds ANSI codes.
	selectedStyle = lipgloss.NewStyle().Reverse(true)

	modalBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("117")).
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
	// The left (sources) frame takes 1/3 of the terminal width and the right
	// (items) frame takes the remaining 2/3. Each border takes 2 chars (left
	// + right), so total border overhead is 4 chars.
	totalInner := m.width - 4
	if totalInner < 4 {
		totalInner = 4
	}
	leftInner := totalInner / 3
	rightInner := totalInner - leftInner

	// Title occupies 1 line; blank separator occupies 1 line; mode line
	// occupies 1 line = 3 lines total.
	const titleOverhead = 3

	// Frame height minus a small margin and the title overhead; inner height
	// subtracts top+bottom border.
	frameHeight := m.height - 2 - titleOverhead
	if frameHeight < 4 {
		frameHeight = 4
	}
	innerHeight := frameHeight - 2
	if innerHeight < 1 {
		innerHeight = 1
	}

	leftContent := m.renderSourceList(leftInner, innerHeight)
	rightContent := m.renderItemList(rightInner, innerHeight)

	var leftStyle, rightStyle lipgloss.Style
	if m.focus == focusSources {
		leftStyle = focusedBorderStyle
		rightStyle = blurredBorderStyle
	} else {
		leftStyle = blurredBorderStyle
		rightStyle = focusedBorderStyle
	}

	leftFrame := renderFrameWithTitle(leftStyle, leftContent, "Feeds", leftInner, innerHeight)
	rightFrame := renderFrameWithTitle(rightStyle, rightContent, "Feed Items", rightInner, innerHeight)

	title := lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render("--=[ newsfed ]=--")
	frames := lipgloss.JoinHorizontal(lipgloss.Top, leftFrame, rightFrame)
	modeLine := m.renderModeLine()

	return lipgloss.JoinVertical(lipgloss.Left, title, "", frames, modeLine)
}

// renderFrameWithTitle renders a styled frame with a custom top border that
// embeds title centered within it. width and height are the inner dimensions
// (excluding borders).
func renderFrameWithTitle(style lipgloss.Style, content, title string, width, height int) string {
	topFg := style.GetBorderTopForeground()
	topLine := buildTitledTopBorder(title, width+2)
	coloredTop := lipgloss.NewStyle().Foreground(topFg).Render(topLine)
	body := style.BorderTop(false).BorderRight(true).BorderBottom(true).BorderLeft(true).
		Width(width).Height(height).Render(content)
	return coloredTop + "\n" + body
}

// buildTitledTopBorder builds a top border line for a rounded frame with the
// given title centered in it. width is the total outer width including
// corners.
func buildTitledTopBorder(title string, width int) string {
	const (
		leftCorner  = "╭"
		fillChar    = "─"
		rightCorner = "╮"
	)
	paddedTitle := " " + title + " "
	titleWidth := utf8.RuneCountInString(paddedTitle)
	innerWidth := width - 2 // subtract corner chars
	if innerWidth <= titleWidth {
		return leftCorner + strings.Repeat(fillChar, max(0, innerWidth)) + rightCorner
	}
	totalFill := innerWidth - titleWidth
	leftFill := totalFill / 2
	rightFill := totalFill - leftFill
	return leftCorner + strings.Repeat(fillChar, leftFill) + paddedTitle + strings.Repeat(fillChar, rightFill) + rightCorner
}

// renderModeLine renders the mode line at the bottom of the screen. It shows
// the current status message in inverse video when one is set; otherwise it
// shows a keyboard shortcut summary.
func (m Model) renderModeLine() string {
	content := m.statusMsg
	if content == "" {
		if m.focus == focusSources {
			content = "[Q]uit  [r]efresh  [R]efresh all  [Tab] Switch  [Enter] Open  [A]dd source"
		} else {
			content = "[Q]uit  [r]efresh  [R]efresh all  [Tab] Switch  [Enter] Open"
		}
	}
	if m.width > 0 {
		content = ansi.Truncate(content, m.width, "")
	}
	return selectedStyle.Width(m.width).Render(content)
}

func (m Model) renderSourceList(width, height int) string {
	if len(m.sources) == 0 {
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Render("No sources.")
	}

	// Each source occupies 1 line. Determine which source the viewport starts
	// from so the cursor stays visible.
	visibleSources := height
	if visibleSources < 1 {
		visibleSources = 1
	}

	startSource := 0
	if m.sourceCursor >= visibleSources {
		startSource = m.sourceCursor - visibleSources + 1
	}

	now := time.Now()
	var lines []string
	for i := startSource; i < len(m.sources); i++ {
		if len(lines) >= height {
			break
		}
		src := m.sources[i]
		prefix := fmt.Sprintf("%d. ", i+1)
		date := formatRelativeLabel(src.LastFetchedAt, now)
		dateLen := utf8.RuneCountInString(date)
		prefixLen := utf8.RuneCountInString(prefix)
		nameMaxWidth := width - prefixLen - dateLen - 1
		if nameMaxWidth < 1 {
			nameMaxWidth = 1
		}
		truncName := ansi.Truncate(src.Name, nameMaxWidth, "...")
		leftPart := prefix + truncName
		padding := width - utf8.RuneCountInString(leftPart) - dateLen
		if padding < 1 {
			padding = 1
		}
		line := leftPart + strings.Repeat(" ", padding) + date

		if i == m.sourceCursor {
			line = selectedStyle.Width(width).Render(line)
		}

		lines = append(lines, line)
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

	// Each item occupies 1 line. Determine how many items fit on screen and
	// which item the viewport starts from so the cursor stays visible.
	visibleItems := height
	if visibleItems < 1 {
		visibleItems = 1
	}

	startItem := 0
	if m.itemCursor >= visibleItems {
		startItem = m.itemCursor - visibleItems + 1
	}

	now := time.Now()
	var lines []string
	for i := startItem; i < len(m.items); i++ {
		if len(lines) >= height {
			break
		}
		item := m.items[i]
		prefix := fmt.Sprintf("%d. ", i+1)
		date := formatRelativeLabel(&item.PublishedAt, now)
		dateLen := utf8.RuneCountInString(date)
		prefixLen := utf8.RuneCountInString(prefix)
		titleMaxWidth := width - prefixLen - dateLen - 1
		if titleMaxWidth < 1 {
			titleMaxWidth = 1
		}
		truncTitle := ansi.Truncate(item.Title, titleMaxWidth, "...")
		leftPart := prefix + truncTitle
		padding := width - utf8.RuneCountInString(leftPart) - dateLen
		if padding < 1 {
			padding = 1
		}
		line := leftPart + strings.Repeat(" ", padding) + date

		if i == m.itemCursor {
			line = selectedStyle.Width(width).Render(line)
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderWithModal(background string) string {
	var modal string
	if m.modal == modalRefreshAll {
		// renderRefreshAllModal builds a custom titled-border box; Place it
		// here alongside all other modals for visual consistency.
		modal = m.renderRefreshAllModal()
	} else {
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
		case modalSourceAdd:
			modalContent = m.renderSourceAddModal()
		}
		modal = modalBorderStyle.Render(modalContent)
	}

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
	fmt.Fprintf(&sb, "Name:             %s\n", src.Name)
	fmt.Fprintf(&sb, "URL:              %s\n", src.URL)
	fmt.Fprintf(&sb, "Type:             %s\n", src.SourceType)
	fmt.Fprintf(&sb, "Enabled:          %v\n", src.IsEnabled())
	fmt.Fprintf(&sb, "Created At:       %s\n", src.CreatedAt.Format("2006-01-02"))
	fmt.Fprintf(&sb, "Updated At:       %s\n", src.UpdatedAt.Format("2006-01-02"))
	if src.LastFetchedAt != nil {
		fmt.Fprintf(&sb, "Last Fetched At:  %s\n", src.LastFetchedAt.Format("2006-01-02"))
	} else {
		sb.WriteString("Last Fetched At:  Never\n")
	}
	if src.PollingInterval != nil {
		fmt.Fprintf(&sb, "Polling Interval: %s\n", *src.PollingInterval)
	} else {
		sb.WriteString("Polling Interval: (default)\n")
	}
	fmt.Fprintf(&sb, "Error Count:      %d\n", src.FetchErrorCount)
	if src.LastError != nil {
		fmt.Fprintf(&sb, "Last Error:       %s\n", *src.LastError)
	}
	return sb.String()
}

func (m Model) renderSourceEditModal() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Name: %s\n", m.editInputs[0].View())
	fmt.Fprintf(&sb, "URL:  %s", m.editInputs[1].View())
	return sb.String()
}

func (m Model) renderSourceAddModal() string {
	var sb strings.Builder
	sb.WriteString("Add Source\n\n")
	fmt.Fprintf(&sb, "Name: %s\n", m.addInputs[0].View())
	fmt.Fprintf(&sb, "URL:  %s", m.addInputs[1].View())
	if m.addDiscovering {
		sb.WriteString("\n\nDiscovering feed...")
	} else if m.statusMsg != "" {
		fmt.Fprintf(&sb, "\n\n%s", m.statusMsg)
	}
	return sb.String()
}

func (m Model) renderDeleteConfirmModal() string {
	if len(m.sources) == 0 {
		return ""
	}
	src := m.sources[m.sourceCursor]

	var sb strings.Builder
	fmt.Fprintf(&sb, "Delete %q? This cannot be undone.\n\n", src.Name)

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

// itemDetailLines builds the full content lines for the item detail modal and
// returns them together with the maximum valid scroll offset. Both the
// renderer and the key handler use this so they share a single source of
// truth for the scroll bounds.
func (m Model) itemDetailLines() (lines []string, maxScroll int) {
	if len(m.items) == 0 {
		return nil, 0
	}
	item := m.items[m.itemCursor]

	// The modal's text width accounts for the border and padding on each
	// side.
	modalWidth := m.width*60/100 - modalBorderOverhead
	if modalWidth < 40 {
		modalWidth = 40
	}

	var sb strings.Builder
	sb.WriteString(wrapField("Title:     ", item.Title, modalWidth) + "\n")
	fmt.Fprintf(&sb, "Published: %s\n", item.PublishedAt.Format("2006-01-02"))
	sb.WriteString(wrapField("URL:       ", item.URL, modalWidth) + "\n")

	if item.Summary != "" {
		plain := stripHTML(item.Summary)
		if plain != "" {
			sb.WriteString("\n")
			sb.WriteString(wordWrap(plain, modalWidth))
		}
	}

	lines = strings.Split(sb.String(), "\n")
	// Drop a trailing empty element produced by a final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// The modal border and padding consume 4 lines vertically (1 border + 1
	// padding on each side), so the visible height is the terminal height
	// minus that overhead.
	visibleHeight := m.height - 4
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	maxScroll = len(lines) - visibleHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	return lines, maxScroll
}

func (m Model) renderItemDetailModal() string {
	lines, maxScroll := m.itemDetailLines()
	if lines == nil {
		return ""
	}

	visibleHeight := m.height - 4
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	scroll := m.itemDetailScroll
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

	// Hard-break any word that is wider than the wrap column so it never
	// produces a line that exceeds width (e.g. a long URL with no spaces).
	var tokens []string
	for _, w := range words {
		runes := []rune(w)
		for len(runes) > width {
			tokens = append(tokens, string(runes[:width]))
			runes = runes[width:]
		}
		tokens = append(tokens, string(runes))
	}

	var lines []string
	current := tokens[0]

	for _, tok := range tokens[1:] {
		if utf8.RuneCountInString(current)+1+utf8.RuneCountInString(tok) <= width {
			current += " " + tok
		} else {
			lines = append(lines, current)
			current = tok
		}
	}
	lines = append(lines, current)

	return strings.Join(lines, "\n")
}

// wrapField formats a labeled field line, hard-wrapping the value so that the
// combined label+value never exceeds width runes. Continuation lines are
// indented to align under the start of the value. This handles values with no
// whitespace (e.g. long URLs) by breaking mid-token at the width boundary.
func wrapField(label, value string, width int) string {
	labelWidth := utf8.RuneCountInString(label)
	indent := strings.Repeat(" ", labelWidth)
	valueWidth := width - labelWidth
	if valueWidth < 10 {
		valueWidth = 10
	}

	runes := []rune(value)
	var chunks []string
	for len(runes) > 0 {
		if len(runes) <= valueWidth {
			chunks = append(chunks, string(runes))
			break
		}
		// Prefer to break at the last space within valueWidth.
		breakAt := valueWidth
		for i := valueWidth - 1; i > 0; i-- {
			if runes[i] == ' ' {
				breakAt = i
				break
			}
		}
		chunks = append(chunks, string(runes[:breakAt]))
		// Skip a space we broke on so it doesn't appear at line start.
		if runes[breakAt] == ' ' {
			runes = runes[breakAt+1:]
		} else {
			runes = runes[breakAt:]
		}
	}

	if len(chunks) == 0 {
		return label
	}
	result := label + chunks[0]
	for _, chunk := range chunks[1:] {
		result += "\n" + indent + chunk
	}
	return result
}

// renderRefreshAllModal renders the Refresh All modal box with its title
// embedded in the top border. It returns the styled box only (no placement);
// the caller (renderWithModal) handles centering via lipgloss.Place.
// Implements Spec 11 section 5.
func (m Model) renderRefreshAllModal() string {
	// Inner content width: min(60, 60% of terminal width) minus
	// border/padding.
	contentWidth := m.width * 60 / 100
	if contentWidth > 60 {
		contentWidth = 60
	}
	contentWidth -= modalBorderOverhead
	if contentWidth < 20 {
		contentWidth = 20
	}

	visible := m.refreshAllVisibleSources()

	content := m.renderRefreshAllContent(contentWidth, visible)

	// Build titled top border then attach the remaining sides.
	totalWidth := contentWidth + modalBorderOverhead
	topLine := buildTitledTopBorder("Refresh All Feeds", totalWidth)
	topColored := lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Render(topLine)

	// Omit Width/Height so lipgloss does not word-wrap the pre-formatted
	// content. Every line produced by renderRefreshAllContent is already
	// padded to exactly contentWidth, so the body renders at the correct size
	// without any reflowing.
	body := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("117")).
		BorderTop(false).
		BorderRight(true).
		BorderBottom(true).
		BorderLeft(true).
		Padding(1, 2).
		Render(content)

	return topColored + "\n" + body
}

// renderRefreshAllContent builds the scrollable source list and summary line
// for the Refresh All modal. contentWidth is the available text width;
// visibleSources is how many source rows fit in the scroll viewport.
func (m Model) renderRefreshAllContent(contentWidth, visibleSources int) string {
	var lines []string

	end := m.refreshAllScroll + visibleSources
	if end > len(m.refreshAllSources) {
		end = len(m.refreshAllSources)
	}

	for i := m.refreshAllScroll; i < end; i++ {
		src := m.refreshAllSources[i]

		indicator := "[ ]"
		result := ""

		if p, ok := m.refreshAllProgress[src.SourceID]; ok {
			switch p.Status {
			case discovery.ProgressFetching:
				indicator = "[~]"
			case discovery.ProgressDone:
				indicator = "[✓]"
				result = fmt.Sprintf("%d new", p.NewItems)
			case discovery.ProgressError:
				indicator = "[!]"
				if p.Error != nil {
					result = ansi.Truncate(p.Error.Error(), 20, "...")
				}
			}
		}

		// Layout: indicator(3) + " "(2) + name + right-aligned result
		resultLen := utf8.RuneCountInString(result)
		nameMax := contentWidth - 3 - 2 - resultLen - 1
		if nameMax < 1 {
			nameMax = 1
		}
		truncName := ansi.Truncate(src.Name, nameMax, "...")
		leftPart := indicator + "  " + truncName
		gap := contentWidth - utf8.RuneCountInString(leftPart) - resultLen
		if gap < 1 {
			gap = 1
		}
		lines = append(lines, leftPart+strings.Repeat(" ", gap)+result)
	}

	// Summary: blank line followed by progress or completion text.
	var summaryLine string
	if m.refreshAllDone {
		totalNew, totalFailed := m.refreshAllSummary()
		total := len(m.refreshAllSources)
		if totalFailed > 0 {
			summaryLine = fmt.Sprintf("Done: %d new item(s) from %d source(s), %d failed",
				totalNew, total, totalFailed)
		} else {
			summaryLine = fmt.Sprintf("Done: %d new item(s) from %d source(s)",
				totalNew, total)
		}
		if m.refreshAllSyncErr != nil {
			summaryLine += fmt.Sprintf(" (error: %v)", m.refreshAllSyncErr)
		}
	} else {
		done := 0
		for _, p := range m.refreshAllProgress {
			if p.Status == discovery.ProgressDone || p.Status == discovery.ProgressError {
				done++
			}
		}
		summaryLine = fmt.Sprintf("Fetching %d/%d sources...", done, len(m.refreshAllSources))
	}

	// Pad every line to contentWidth so the border renders at a consistent
	// width without lipgloss needing to reflow or word-wrap the content.
	padLine := func(s string) string {
		n := utf8.RuneCountInString(s)
		if n < contentWidth {
			return s + strings.Repeat(" ", contentWidth-n)
		}
		return s
	}
	for i, l := range lines {
		lines[i] = padLine(l)
	}
	lines = append(lines, padLine(""))          // blank separator
	lines = append(lines, padLine(summaryLine)) // summary

	return strings.Join(lines, "\n")
}

// formatRelativeLabel formats a nullable time as a parenthetical label:
// "(never)" if nil, "(today)" if the time is today, or "(X ago)" otherwise.
func formatRelativeLabel(t *time.Time, now time.Time) string {
	if t == nil {
		return "(never)"
	}
	rel := relativeDate(*t, now)
	if rel == "today" {
		return "(today)"
	}
	return "(" + rel + " ago)"
}

// relativeDate returns a compact relative date string such as "1d", "2mo3d",
// or "1y2mo4d" representing the calendar distance from t to now. If t is not
// before now, it returns "0d".
func relativeDate(t time.Time, now time.Time) string {
	if !t.Before(now) {
		return "today"
	}

	cur := t
	y, mo, d := 0, 0, 0

	for next := cur.AddDate(1, 0, 0); !next.After(now); next = cur.AddDate(1, 0, 0) {
		y++
		cur = next
	}
	for next := cur.AddDate(0, 1, 0); !next.After(now); next = cur.AddDate(0, 1, 0) {
		mo++
		cur = next
	}
	for next := cur.AddDate(0, 0, 1); !next.After(now); next = cur.AddDate(0, 0, 1) {
		d++
		cur = next
	}

	var sb strings.Builder
	if y > 0 {
		fmt.Fprintf(&sb, "%dy", y)
	}
	if mo > 0 {
		fmt.Fprintf(&sb, "%dmo", mo)
	}
	if d > 0 || (y == 0 && mo == 0) {
		if y == 0 && mo == 0 && d == 0 {
			return "today"
		}
		fmt.Fprintf(&sb, "%dd", d)
	}
	return sb.String()
}
