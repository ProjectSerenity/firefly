.PHONY: all
all: kcheck

kcheck: $(wildcard *.go) go.mod go.sum $(wildcard cc/*.go)
	go test
	go build
