.PHONY: build
build:
	go build -gcflags='-e' .
	go build -gcflags='-e' ./cmd/getres

.PHONY: build-all
build-all: build
	go build -gcflags='-e' ./cmd/dstsrvs

.PHONY: test
test:
	go test ./...

.PHONY: tags
tags:
	gotags -f tags -R .

.PHONY: vet
vet:
	go vet ./...

.PHONY: lint
lint:
	golint ./...

.PHONY: clean
clean:
	rm -f serinin dstsrvs getres
	rm -f *.exe *.exe~
