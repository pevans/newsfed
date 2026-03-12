# Changelog

All notable changes to gocomments will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.1] - 2026-03-12

### Added

- The pin command (already existent in the CLI) was added to the text UI.
  Press 'p' to pin or unpin a news item. Pinning a news item prevents it from
  being removed from the screen.

### Changed

- When the refresh-all command completes, the relative times when feeds were
  last fetched are replaced with the number of new items that were fetched. If
  nothing was fetched, nothing will be shown.

## [0.2.0] - 2026-03-03

### Added

- Added a "Refresh all feeds" command to the text UI, triggered when you hit
  'R' (upper case). Pops out a modal with all the feeds and shows their
  progress along with any issues encountered.

### Changed

- Feeds now render on one line with the relative date for when they were last
  fetched
- Frames now have titles embedded into their top borders
- Refreshing a feed now only works when you press 'r' (lower case)

### Fixed

- Refreshing a feed now updates its last-updated date

## [0.1.0] - 2026-03-01

This is the first release of newsfed, a small news feed reader.

[0.2.1]: https://github.com/pevans/newsfed/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/pevans/newsfed/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/pevans/newsfed/releases/tag/v0.1.0

