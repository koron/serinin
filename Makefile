.PHONY: build
build:
	go build

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
