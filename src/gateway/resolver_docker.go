package main

import (
	"errors"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"

	"io"
	"log"
	"strings"
)

type ResolverDocker struct {
	portMappings map[string]uint16
	baseHostname string
	gatewayIp    string
	client       *client.Client
}

func NewDockerResolver(apiVersion string) *ResolverDocker {
	cli, err := client.NewEnvClient()
	if len(apiVersion) > 0 {
		cli, err = client.NewClientWithOpts(client.WithVersion(apiVersion))
	}
	if err != nil {
		panic(err)
	}
	docker := &ResolverDocker{}
	docker.client = cli

	go docker.listenEvents()
	return docker
}

func (d *ResolverDocker) SetBaseHostname(hostname string) {
	d.baseHostname = hostname
}
func (d *ResolverDocker) SetGatewayIp(gatewayip string) {
	d.gatewayIp = gatewayip
}
func (d *ResolverDocker) FetchPortMappings() {
	containers, err := d.client.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		panic(err)
	}

	d.portMappings = make(map[string]uint16)
	for _, container := range containers {
		for _, port := range container.Ports {
			if port.PrivatePort == 80 && port.Type == "tcp" && port.PublicPort > 0 && port.PrivatePort != port.PublicPort {
				for _, name := range container.Names {
					d.portMappings[name[1:]] = port.PublicPort
				}
			}
		}
	}
	fmt.Println(d.portMappings)
}
func (d *ResolverDocker) listenEvents() {
	messages, errs := d.client.Events(context.Background(), types.EventsOptions{})
	for {
		select {
		case err := <-errs:
			if err != nil && err != io.EOF {
				log.Fatal(err)
			}
			return
		case e := <-messages:
			if e.Type == events.NetworkEventType {
			}
			d.FetchPortMappings()
		}
	}
}

func (d *ResolverDocker) GetDestinationHostPort(srcHostPort string) (dstHostPort string, err error) {
	srcHost := strings.Split(srcHostPort, ":")[0]
	sublevel := srcHost[:len(srcHost)-len(d.baseHostname)-1]
	sublevels := strings.Split(sublevel, ".")
	fmt.Println(sublevel, sublevels)
	host := sublevels[len(sublevels)-1]
	for name, dstHostPort := range d.portMappings {
		if strings.Split(name, "_")[0] == host {
			return fmt.Sprintf("%s:%d", d.gatewayIp, dstHostPort), nil
		}
	}
	return "", errors.New(fmt.Sprintf("No destination Found for host '%s' (%s)", srcHost, host))
}
