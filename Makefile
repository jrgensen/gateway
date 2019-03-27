NAME=gateway
REPO=jrgensen/$(NAME)
WORKDIR=/go/src/$(NAME)
DOCKER=docker run --rm -ti -v `pwd`/src:/go/src/$(NAME) -w $(WORKDIR) --env CGO_ENABLED=0 golang:1.12

compile:
	$(DOCKER) go get -t ./...
	$(DOCKER) go build -a -installsuffix cgo .

build: 
	docker build -t $(REPO):docker .

push:
	docker push $(REPO)

run:
	docker run -d -p 80:80 --volume /var/run/docker.sock:/var/run/docker.sock --restart always -e HOSTNAME=local.pnorental.com -e DESTINATION_RESOLVER=docker --name $(NAME) $(REPO)

.PHONY: compile build watch dependencies test init
