# Changelog

All notable changes to gocomments will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[0.2.0]: https://github.com/pevans/newsfed/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/pevans/newsfed/releases/tag/v0.1.0

