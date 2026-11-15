.PHONY: download-migrations generate build run

download-migrations:
	go run ./tools/download-migrations

generate: download-migrations
	sqlc generate

build:
	go build -o build/server .

run:
	go run .