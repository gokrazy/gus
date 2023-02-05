.PHONY: all test run run-router7 postgres

export POSTGRES_HOST = 127.0.0.1
export POSTGRES_PORT = 5433
export POSTGRES_USER = postgres
export POSTGRES_PASSWORD = mysecretpassword
export POSTGRES_DBNAME = postgres

all:
	go install ./cmd/...

test:
	go test -count=1 -v ./...

run: test all
	gus-server

run-router7:
	cd cmd/gus-server && GOARCH=amd64 gok -i router7 run

postgres:
	docker kill gus-postgres || true
	docker run --rm -it -d --name gus-postgres \
		-e POSTGRES_USER=$$POSTGRES_USER \
		-e POSTGRES_PASSWORD=$$POSTGRES_PASSWORD \
		-p $$POSTGRES_PORT:5432 postgres
