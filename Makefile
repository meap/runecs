build:
	go build -o bin/runecs main.go

lint:
	golangci-lint run

release:
	@scripts/release.sh "$(V)"

.PHONY: release
