.PHONY: all test run run-router7

all:
	go install ./cmd/...

_bin/initpg: internal/cmd/initpg/initpg.go
	go build -o $@ ./internal/cmd/initpg

test: _bin/initpg
	_bin/initpg -- go test -v ./...

run: test all
	gus-server

run-router7:
	cd cmd/gus-server && GOARCH=amd64 gok -i router7 run
