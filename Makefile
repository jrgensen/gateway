NAME=gateway
REPO=jrgensen/$(NAME)
WORKDIR=/go/src/$(NAME)
DOCKER=docker run --rm -ti -v `pwd`:/go -w $(WORKDIR) --env CGO_ENABLED=0 golang:1.10

compile: dependencies
	$(DOCKER) go build -a -installsuffix cgo .

build: compile
	docker build -t $(REPO) .

push:
	docker push $(REPO)

watch: dependencies
	$(DOCKER) ginkgo watch

dependencies:
	test -s bin/ginkgo || ( $(DOCKER) go get github.com/onsi/ginkgo/ginkgo; )
	$(DOCKER) ginkgo bootstrap || true;
	$(DOCKER) go get -t ./...

test: dependencies
	$(DOCKER) go test ./...

fmt:
	$(DOCKER) go fmt ./...

run:
	docker run -d --rm -p 80:80 --volume /var/run/docker.sock:/var/run/docker.sock -e HOSTNAME=local.pnorental.com -e DOCKER_PORT_PROXY=1 -e DOCKER_API_VERSION=1.38 --name $(NAME) $(REPO)

.PHONY: compile build watch dependencies test init
