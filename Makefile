.PHONY: build run release

GITHUB_TOKEN ?= $(shell gh auth token)

build:
	go build -o falcode .

run:
	go run .

release:
	@if [ -z "$(TAG)" ]; then echo "Usage: make release TAG=vX.Y.Z"; exit 1; fi
	git tag $(TAG)
	git push origin $(TAG)
	GITHUB_TOKEN=$(GITHUB_TOKEN) goreleaser release --clean
