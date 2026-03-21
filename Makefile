.PHONY: build run test vet check clean

BINARY_DIR := dist
BINARY := $(BINARY_DIR)/feishu

check:
	$(MAKE) test
	$(MAKE) vet

build:
	mkdir -p $(BINARY_DIR)
	go build -o $(BINARY) ./cmd/feishu

run:
	go run ./cmd/feishu

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -rf $(BINARY_DIR)
	rm -f feishu.exe
