.PHONY: all
all: kcheck

kcheck: $(wildcard *.go) go.mod go.sum
	go test
	go build
