export GOTOOLCHAIN := local

VERSION ?= $(shell awk 'NR==1{gsub(/[()]/,""); print $$2; exit}' debian/changelog 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

all: b

b:
	mkdir -p build
	go build -ldflags='$(LDFLAGS)' -o build/wclip ./src

test:
	go test ./src/...

fmt:
	for f in src/*.go; do go fmt $$f && sed -i -e 's/	/  /g' $$f; done

# https://blog.codeship.com/building-minimal-docker-containers-for-go-applications/
docker:
	mkdir -p build
	CGO_ENABLED=0 GOOS=linux go build -ldflags='$(LDFLAGS)' -o build/wclip.docker ./src
	docker build -t wclip-go .
