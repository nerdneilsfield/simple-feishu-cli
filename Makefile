.PHONY: test vet check

check:
	$(MAKE) test
	$(MAKE) vet

test:
	go test ./...

vet:
	go vet ./...
