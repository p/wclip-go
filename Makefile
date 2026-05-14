export GOTOOLCHAIN := local

# Version, in priority order:
#   1. git describe --tags --dirty            (clone with at least one tag)
#   2. dpkg-parsechangelog -SVersion          (Debian source tree)
#   3. awk on debian/changelog                (tarball without dpkg-dev)
#   4. literal "dev"                          (last-ditch fallback)
# Leading "v" from git tags is stripped to match Debian/semver-bare style.
VERSION ?= $(shell \
  git describe --tags --dirty 2>/dev/null | sed 's/^v//' | grep . \
  || dpkg-parsechangelog -SVersion 2>/dev/null \
  || awk 'NR==1{gsub(/[()]/,""); print $$2; exit}' debian/changelog 2>/dev/null \
  || echo dev)

# Short commit SHA and commit time. Using the committer date (not
# wall-clock build time) keeps builds of the same source deterministic.
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE   ?= $(shell git log -1 --format=%cI 2>/dev/null || echo unknown)

LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

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
