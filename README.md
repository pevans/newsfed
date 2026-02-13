# newsfed

newsfed is a small, terminal-based tool that tracks news feeds (RSS and Atom)
or websites-that-look-like-news-feeds for updates. All of its metadata and
news items are saved to the computer that runs newsfed.

newsfed is designed initially with a command-line interface. While that works
well as a proof of concept -- and, theoretically, for automation -- it's not
the most usable tool for human beings. I plan to build out a terminal user
interface shortly.

## Installation

You can install newsfed by cloning the repository and running:

```bash
# If you have `just` installed
just build

# If you don't have `just` installed
go build -o dist/newsfed ./cmd/newsfed
```

## CLI Usage

Before you can use newsfed's CLI, you must first initialize it:

```bash
newsfed init
```

This command will create:

- `~/.newsfed/config.yaml`, a configuration file
- `~/.newsfed/metadata.db`, a SQLite database containing metadata (such as
  what source feeds to read)
- `~/.newsfed/feed/`, the directory that holds news items as JSON files

newsfed cannot function without these files.

### Adding a feed

To add a feed, you issue a command like so:

```bash
newsfed sources add \
  -type=rss \
  -url=https://awesomenewssitethatdoesntexist.com/ \
  -name="not a real news site"
```

### Reading from a feed

To read from a feed, see below:

```bash
# fetch updates from your feeds
newsfed sync

# list all recent news
newsfed list

# show a single news item
newsfed show <id>
````

## Contributing

I am not accepting pull requests at this time.

## License

See LICENSE for the project's license.
