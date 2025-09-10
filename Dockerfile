FROM golang:1.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ENV CGO_ENABLED=0
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /app/filefinder ./cmd/main.go

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /found_files
WORKDIR /app

COPY --from=builder /app/filefinder /usr/local/bin/filefinder

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/filefinder"]
CMD []
