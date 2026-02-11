# Run white box (unit) tests
utest:
    go test ./...

# Run black box tests
btest:
    tests/run-all-tests.sh

build:
    go build -o dist/newsfed ./cmd/newsfed
