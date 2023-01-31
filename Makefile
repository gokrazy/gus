.PHONY: all test run

all:
	go install ./cmd/...

test:
	go test -v ./...

run: test all
	gus-server
