.PHONY: build test vet fmt golden-unix clean

build:
	go build -o bin/peek ./cmd/peek

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -s -w .

golden-unix:
	@echo "golden-unix target will be filled in Task 9"

clean:
	rm -rf bin/ dist/
