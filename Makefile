.PHONY: test vet check

test:
	go test ./...

vet:
	go vet ./...

check:
	$(MAKE) test
	$(MAKE) vet
