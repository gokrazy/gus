.PHONY: all test run run-router7

all:
	go install ./cmd/...

test:
	go test -v ./...

run: test all
	gus-server

run-router7:
	cd cmd/gus-server && GOARCH=amd64 gok -i router7 run
