FROM golang:1.23.1-alpine AS build

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod tidy

COPY . .

RUN go build -ldflags="-s -w" -o /app/runecs .

FROM alpine:latest

WORKDIR /app/

RUN apk --no-cache add ca-certificates

COPY --from=build /app/runecs .

RUN adduser -D appuser
USER appuser

ENTRYPOINT ["./runecs"]
