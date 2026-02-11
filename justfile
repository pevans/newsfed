# Run white box (unit) tests
utest:
    go test ./...

# Run black box tests
btest:
    tests/run-all-tests.sh

build:
    go build -o dist/newsfed ./cmd/newsfed
    go build -o dist/newsfed-metadata-api ./cmd/metadata-api
    go build -o dist/newsfed-discover ./cmd/newsfed-discover
    go build -o dist/newsfed-newsfeed-api ./cmd/newsfeed-api
