---
Specification: 9
Title: Text User Interface
Drafted At: 2026-02-27
Authors:
  - Peter Evans
---

# 1. Overview

Newsfed should provide a text user interface (TUI) in addition to its
command-line interface. Such an interface would allow users to manage their
sources, read news items, fetch updates, and launch browser windows to follow
the URL sources in items.

The TUI is launched by running `newsfed tui` or by running `newsfed` with no
arguments.

# 2. Main wireframe

When Newsfed's TUI opens, a title bar is shown at the top of the screen:

```
--=[ newsfed ]=--
```

The title is centered horizontally. One blank line separates the title from the
frames below.

Below the title, the screen is split into two frames.

- On the left is a sources frame that occupies roughly one third of the
  terminal width. It renders a list of all sources.
  - Users would be able to select a source to view or manage.
  - There is always one source that is selected; by default, it's the first
    source in the list.
  - Hitting enter opens a floating modal (that takes up the center of the
    screen) with options edit or delete the source.
- On the right is a news items frame that occupies the remaining two thirds
  of the terminal width. In this frame is each news item of the selected
  source.
    - Hitting enter opens a floating modal (that again takes up the center of
      the screen) with the full details of the news item.
- Each frame should have a single lined border with rounded corners.
- The sources frame displays the label "Feeds" centered in its top border.
- The news items frame displays the label "Feed Items" centered in its top
  border.

Details for how to render the items of the frames and modals are given below.

# 3. Source frame

The source frame contains a list of news sources. Each item in the frame is
rendered on a single line. The name is shown on the left and the last-fetched
date is shown on the right, separated by spaces to fill the full width of the
frame:

<item number>. <name of source>          (<relative date> ago)

Here's a filled-in example:

3. Fun Times Ahead                       (3y2mo1d ago)

The item number is the index in the sources list; i.e., the first item is 1;
the second item is 2; and so forth. The sources should be sorted
alphabetically.

The relative date uses the same format as news items (see Section 4): calendar
years (y), months (mo), and days (d), showing only non-zero components. A
source fetched today shows "(today)". A source that has never been fetched
shows "(never)" in place of a relative date.

Before rendering, any run of one or more whitespace characters (spaces, tabs,
newlines) in the name is collapsed to a single space.

If the name is too long to fit alongside the date, it is truncated with "..."
appended to the end of what remains. Enough text is truncated to leave at
least one space between the name and the date.

No source has any text that comprises a visual border between it and other
sources.

A selected source should be rendered with inverted video.

If there are no sources, the source frame displays the message "No sources."
centered vertically and horizontally within the frame.

# 4. News items frame

The news items frame contains a list of items belonging to the selected source.
Each item in the frame is rendered on a single line. The title is shown on the
left and the published date is shown on the right, separated by spaces to fill
the full width of the frame:

<item number>. <title of item>          (<relative date> ago)

Here's a filled-in example:

2. Scientists Discover Talking Badger               (3y2mo1d ago)

The item number is the index in the items list; i.e., the first item is 1, the
second item is 2, and so forth. The items should be sorted in reverse
chronological order, so that the most recently published item appears first.

The relative date uses calendar years (y), months (mo), and days (d), showing
only the non-zero components from most to least significant. For example:

- An item published one day ago shows "(1d ago)"
- An item published two days ago shows "(2d ago)"
- An item published one month and two days ago shows "(1mo2d ago)"
- An item published three years, two months, and one day ago shows
  "(3y2mo1d ago)"
- An item published today shows "(today)"

Before rendering, any run of one or more whitespace characters (spaces, tabs,
newlines) in the title is collapsed to a single space.

If the title is too long to fit alongside the date, it is truncated with "..."
appended to the end of what remains. Enough text is truncated to leave at
least one space between the title and the date.

No item has any text that comprises a visual border between it and other items.

A selected item should be rendered with inverted video.

If the selected source has no items, the news items frame displays the message
"<No items>" centered vertically and horizontally within the frame.

# 5. Source management modal

When a user hits enter on a selected source, a floating modal appears in the
center of the screen.

The modal should show every field of the source in the form of "field: value".

The modal also presents two options:

- Edit
- Delete

The user moves between options using the up and down arrow keys. One option is
always highlighted with inverted video; by default it is "Edit". The user
confirms a selection by pressing enter. The user dismisses the modal without
taking action by pressing escape.

## 5.1. Edit

When the user selects "Edit", the modal's content is replaced with an edit form
containing two labeled fields:

- Name: <current name>
- URL:  <current URL>

The cursor begins in the Name field. The user moves between fields using tab.
The user saves the changes by pressing enter, which closes the modal and updates
the source in place. The user cancels without saving by pressing escape, which
closes the modal and leaves the source unchanged.

## 5.2. Delete

When the user selects "Delete", the modal's content is replaced with a
confirmation prompt:

Delete "<source name>"? This cannot be undone.

[ Yes ]   [ No ]

The user moves between "Yes" and "No" using the left and right arrow keys. "No"
is highlighted by default. The user confirms by pressing enter. If the user
confirms "Yes", the source is deleted and the modal closes; the next source in
the list becomes selected, or if none remains, the source frame shows the "No
sources." message. If the user confirms "No", the modal closes without changes.
The user may also press escape to dismiss the prompt without deleting.

# 6. News item detail modal

When a user hits enter on a selected news item, a floating modal appears in the
center of the screen displaying the full details of the item. The modal renders
the following fields:

Title:     <title>
Published: <published at date>
URL:       <url>

<description>

The description is rendered below the labeled fields, separated by a blank line.
Long lines in the description are wrapped to fit within the modal's width. If
the description is absent, it is omitted entirely along with the blank line.

Since news item descriptions may include HTML code, newsfed should omit those
and simply show the plain text of the description.

The user closes the modal by pressing escape.

The user opens the item's URL in the system's default browser by pressing "o"
while the modal is open. After launching the browser, the modal remains open.

# 7. Navigation and keybindings

## 7.1. Frame navigation

The two frames are independently focusable. The focused frame is the one whose
border is highlighted in light blue (xterm-256 color 117). At startup, the
source frame has focus.

The user moves focus between frames using the tab key.

## 7.2. Within a frame

Within the focused frame, the user moves the selection up and down using the up
and down arrow keys. The selection wraps: pressing down on the last item moves
the selection to the first item, and pressing up on the first item moves it to
the last.

Pressing enter on a selected item opens that item's modal, as described in
sections 5 and 6.

We should also support vim bindings for movement: 'j' for down, and 'k' for
up.

## 7.3. Global keybindings

The following keybindings are active at all times unless a modal is open:

- r -- fetch updates for the currently selected source. While the fetch is in
  progress the source frame displays a brief status message at the bottom of the
  frame. On completion, the news items frame refreshes to show any new items.
- q -- quit the TUI and return to the shell.

# 9. Mode line

A mode line is displayed at the bottom of the screen in inverse video. It spans
the full terminal width.

By default, the mode line shows a keyboard shortcut summary:

```
[Q]uit  [R]efresh  [Tab] Switch  [Enter] Open
```

Whenever a status message is active -- for example, while a source is being
fetched or after a fetch completes -- the mode line displays that message
instead of the shortcut summary. When the status message clears (for example,
when the user navigates away), the mode line reverts to the shortcut summary.

# 8. Date formatting

News item published dates and source last-fetched dates are both rendered as
relative dates using the format described in Section 4. If a source has never
been fetched, its date shows "(never)" in place of a relative date.

# 10. Add Source

When the source frame is focused, the mode line includes an `[A]dd source`
entry in the keyboard shortcut summary.

Pressing `a` while the source frame is focused opens an "Add Source" modal.
Pressing `a` while the news items frame is focused has no effect.

The modal presents three labeled fields:

- Name: \<input\>
- URL:  \<input\>
- Type: \<input\>

The cursor begins in the Name field. The user moves between fields using tab.

The user creates the source by pressing enter. All three fields are required;
if any are empty when enter is pressed, a status message is displayed and the
modal remains open. If the type value is not one of `rss`, `atom`, or
`website`, an error is shown and the modal remains open.

On success, the modal closes and the new source appears in the source list
with the cursor positioned on it. The source is enabled immediately upon
creation.

The user dismisses the modal without adding a source by pressing escape.

> **Note:** Spec 10 (Feed Auto-Discovery), Section 7.1 updates this modal to
> remove the Type field and add auto-discovery behavior.

# 11. Pin / Unpin

When the news items frame is focused, pressing `P` (uppercase or lowercase)
toggles the pinned state of the currently selected news item. If the item is
unpinned, it becomes pinned; if it is already pinned, it becomes unpinned.

Pinned items are displayed with a pin indicator prepended to their title in the
news items frame:

```
2. [📌] Scientists Discover Talking Badger          (3y2mo1d ago)
```

Unpinned items have no such indicator.

Pinned items are always sorted above unpinned items, regardless of published
date. Within the pinned group and within the unpinned group, items retain the
default reverse-chronological sort order.

The pin state is persisted so that it survives restarts of the TUI.

Pressing `P` while the source frame is focused has no effect. Pressing `P`
while a modal is open has no effect.

When the news items frame is focused, the mode line includes `[P]in` in the
keyboard shortcut summary.
