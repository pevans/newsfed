# Run white box (unit) tests
utest:
    go test ./...

# Run black box tests
btest:
    ./tests/run-all.sh
