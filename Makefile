all: b

b:
	mkdir -p tmp
	go build -o tmp/wclip src/server.go src/robots.go

fmt:
	for f in src/*.go; do go fmt $$f && sed -i -e 's/	/  /g' $$f; done

# https://blog.codeship.com/building-minimal-docker-containers-for-go-applications/
docker:
	mkdir -p tmp
	CGO_ENABLED=0 GOOS=linux go build -o tmp/wclip.docker src/server.go src/robots.go
	docker build -t wclip-go .
