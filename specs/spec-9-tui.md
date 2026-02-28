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

# 2. Main wireframe

When Newsfed's TUI opens, it should be split roughly into halves.

- In the first half, on the left, is a frame that renders a list of all
  sources.
  - Users would be able to select a source to view or manage.
  - There is always one source that is selected; by default, it's the first
    source in the list.
  - Hitting enter opens a floating modal (that takes up the center of the
    screen) with options edit or delete the source.
- In the final half is a second frame. In this frame is each news item
  of the selected source.
    - Hitting enter opens a floating modal (that again takes up the center of
      the screen) with the full details of the news item.
- Each frame should have a single lined border with rounded corners.

Details for how to render the items of the frames and modals are given below.

# 3. Source frame

The source frame contains a list of news sources. Each item in the frame is
rendered using the following form:

<item number>. <name of source> (<type of source>)
Last updated: <updated at date>

Here's a filled-in example:

3. Fun Times Ahead (rss)
Last updated: 1999-01-01

The item number is the index in the sources list; i.e., the first item is 1;
the second item is 2; and so forth. The sources should be sorted
alphabetically.

Each source therefore comprises two lines each of text. If any of the lines
cannot be rendered fully, then they are truncated with "..." appended to the
end of what remains. Enough text must be truncated so that "..." can be
rendered without being cut off.

No source has any text that comprises a visual border between it and other
sources.

A selected source should be rendered with inverted video.

If there are no sources, the source frame displays the message "No sources."
centered vertically and horizontally within the frame.

# 4. News items frame

The news items frame contains a list of items belonging to the selected source.
Each item in the frame is rendered using the following form:

<item number>. <title of item>
Authors: <authors of the item>
Published: <published at date>

Here's a filled-in example:

2. Scientists Discover Talking Badger
Authors: Dan Serious, Joni Smitchell
Published: 1999-01-15

The item number is the index in the items list; i.e., the first item is 1, the
second item is 2, and so forth. The items should be sorted in reverse
chronological order, so that the most recently published item appears first.

Each item therefore comprises three lines of text. The same truncation rules as
the source frame apply: lines that cannot be rendered fully are truncated with
"..." appended to the end of what remains.

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
border is highlighted. At startup, the source frame has focus.

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

# 8. Date formatting

Dates are rendered in the format YYYY-MM-DD. If a source has never been
fetched, its "Last updated" field displays "Never" in place of a date.
