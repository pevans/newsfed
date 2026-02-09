# Run white box (unit) tests
utest:
    go test ./...

# Run black box tests
btest:
    @for d in ./tests.v2/*; do \
        if [ -f $d/run-all.sh ]; then \
            bash $d/run-all.sh; \
        fi; \
    done

build:
    go build -o dist/newsfed ./cmd/newsfed
    go build -o dist/newsfed-metadata-api ./cmd/metadata-api
    go build -o dist/newsfed-discover ./cmd/newsfed-discover
    go build -o dist/newsfed-newsfeed-api ./cmd/newsfeed-api
