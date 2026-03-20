.PHONY: build run test vet check clean

BINARY := feishu

check:
	$(MAKE) test
	$(MAKE) vet

build:
	go build -o $(BINARY) ./cmd/feishu

run:
	go run ./cmd/feishu

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY) feishu.exe
