build:
	go build -o bin/runecs main.go

lint:
	golangci-lint run
