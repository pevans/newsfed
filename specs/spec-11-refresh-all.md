---
Specification: 11
Title: Refresh All Feeds
Drafted At: 2026-03-02
Authors:
  - Peter Evans
---

# 1. Overview

The TUI currently supports refreshing a single source at a time via the `r`
keybinding (Spec 9). This specification describes a "Refresh All" operation
that fetches every enabled source, displays live progress in a floating modal
window, and reports per-source results including new item counts.

# 2. Keybinding

The `R` (shift-r) key triggers the Refresh All operation when no modal is open.
The operation is ignored if a refresh-all is already in progress.

The existing `r` keybinding for single-source refresh remains unchanged.

# 3. Concurrency

Refresh All reuses the existing `DiscoveryService.SyncSources` concurrency
model (Spec 2, Section 2.4; currently defaulting to 5 concurrent fetches).
Sources are fetched in parallel up to the concurrency limit, with a semaphore
controlling how many fetches are in flight at any given time.

Per-domain rate limiting (Spec 2, Section 2.4) continues to apply. No changes
to the concurrency or rate-limiting defaults are required by this spec.

# 4. Progress reporting

## 4.1. Progress channel

`SyncSources` must accept an optional progress channel that receives per-source
updates as each fetch begins and completes. The channel carries messages of the
form:

```go
type SourceProgress struct {
    Source      sources.Source
    Status      string // "fetching", "done", "error"
    NewItems    int    // number of new items discovered (0 while fetching)
    Error       error  // nil unless Status is "error"
}
```

When the channel is nil, `SyncSources` behaves exactly as it does today.

## 4.2. Ordering

Progress messages arrive in whatever order fetches start and complete --
sources that finish quickly will report before slower ones. The modal must
handle out-of-order updates gracefully.

# 5. Refresh All modal

## 5.1. Appearance

The modal is a bordered, floating overlay centered on screen -- consistent with
other modals in the TUI (Spec 9, Section 4). Its title, centered in the top
border, is `Refresh All Feeds`.

The modal is wide enough to show source names and result summaries without
excessive truncation. A reasonable default is 60 columns or 60% of the terminal
width, whichever is smaller.

The modal height should accommodate all enabled sources when possible, up to a
maximum of 80% of the terminal height. If there are more sources than fit, the
list scrolls (see Section 5.4).

## 5.2. Source list

Each enabled source occupies one line in the modal. Lines are formatted as:

```
<status-indicator>  <source-name>              <result>
```

The status indicator is a short text token reflecting the source's current
state:

| State    | Indicator | Result column         |
|----------|-----------|-----------------------|
| Pending  | `[ ]`     | (empty)               |
| Fetching | `[~]`     | (empty)               |
| Done     | `[✓]`     | `N new`               |
| Error    | `[!]`     | truncated error text  |

The source name is left-aligned and truncated with an ellipsis if it exceeds
the available space. The result column is right-aligned.

Sources are listed in the same alphabetical order as the sources frame.

## 5.3. Summary line

Below the source list, separated by a blank line, a summary line shows
aggregate progress:

While in progress:

```
Fetching N/M sources...
```

Where `N` is the number of sources that have finished (done or error) and `M`
is the total number of enabled sources.

After all fetches complete:

```
Done: N new item(s) from M source(s), F failed
```

The failed count is omitted when zero:

```
Done: N new item(s) from M source(s)
```

## 5.4. Scrolling

If the source list exceeds the visible area, the list scrolls to keep the most
recently updated source visible. The user may also scroll manually with
`j`/`k` or arrow keys while the modal is open.

## 5.5. Dismissal

While the refresh is in progress, pressing `Esc` keeps the modal open -- the
user cannot dismiss it until all fetches complete. This prevents the user from
accidentally hiding progress and losing visibility into a running operation.

Once all fetches have completed, the user dismisses the modal by pressing `Esc`
or `q`. Upon dismissal:

- The sources frame and items frame reload to reflect any newly fetched items.
- The mode line briefly shows a summary (e.g., `Refreshed all: 12 new item(s)`)
  before reverting to the default shortcut hints.

# 6. Edge cases

## 6.1. No enabled sources

If there are no enabled sources, pressing `R` displays a status message in the
mode line -- `No enabled sources` -- and does not open the modal.

## 6.2. Already refreshing

If a refresh-all operation is already in progress (the modal is open), pressing
`R` again is ignored.

## 6.3. Single-source refresh during refresh-all

While the refresh-all modal is open, the `r` single-source keybinding is
ignored to avoid conflicting concurrent fetches of the same source.

# 7. Mode line

The mode line (Spec 9, Section 5) should include `R: refresh all` in its
shortcut hints when no modal is open. This sits alongside the existing
keybinding hints.
