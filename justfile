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
    gofmt -l .
    gocomments -l .

format:
    gofmt -w .
    gocomments -w .

build:
    go build -o dist/newsfed ./cmd/newsfed

install: build
    mv dist/newsfed ~/bin
