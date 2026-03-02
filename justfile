# Run white box (unit) tests
utest:
    go test ./...

# Run black box tests
btest:
    tests/run-all-tests.sh

cicd-test: lint format-list utest btest

lint:
    golangci-lint run ./...

format-list:
    @# Note that gofmt -l does not return an error code even if it finds
    @# differences; thus we also assert that the results are an empty string
    gofmt -l . && test -z "$(gofmt -l .)"
    gocomments -l .

format:
    gofmt -w .
    gocomments -w .

build:
    go build -o dist/newsfed ./cmd/newsfed

install: build
    mv dist/newsfed ~/bin
