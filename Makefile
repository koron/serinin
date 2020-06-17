.PHONY: build
build:
	go build -gcflags='-e' .
	go build -gcflags='-e' ./cmd/dstsrvs
	go build -gcflags='-e' ./cmd/getres

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
