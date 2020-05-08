.PHONY: all
all: imgbuild

imgbuild: $(wildcard *.go) go.mod go.sum
	go test
	go build
