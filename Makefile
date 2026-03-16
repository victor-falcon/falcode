.PHONY: build run test release

build:
	go build -o falcode .

run:
	go run .

test:
	go test ./...

release:
	@if [ -z "$(TAG)" ]; then echo "Usage: make release TAG=vX.Y.Z"; exit 1; fi
	git tag $(TAG)
	git push origin $(TAG)
