APP := filefinder
SRC := ./cmd/main.go
BIN := ./bin

.PHONY: build test clean

build: clean
	@mkdir -p $(BIN)
	go build -o $(BIN)/$(APP) $(SRC)

test:
	go test -v ./...

clean:
	rm -rf $(BIN)
