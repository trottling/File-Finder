FROM golang:1.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o filefinder ./cmd/main.go

FROM gcr.io/distroless/static-debian11

WORKDIR /app
COPY --from=builder /app/filefinder /usr/local/bin/filefinder

RUN mkdir -p /found_files

ENTRYPOINT ["/usr/local/bin/filefinder"]

CMD []
