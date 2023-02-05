.PHONY: all test run run-router7 postgres

all:
	go install ./cmd/...

test:
	go test -v ./...

run: test all
	gus-server

run-router7:
	cd cmd/gus-server && GOARCH=amd64 gok -i router7 run

postgres:
	docker run --rm -it --name some-postgres -e POSTGRES_PASSWORD=mysecretpassword -p 5432:5432 postgres
