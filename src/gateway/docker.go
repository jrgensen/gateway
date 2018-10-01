package main

import (
	"errors"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"

	"net/http"
	"net/http/httputil"
	"strings"
)

type Docker struct {
	portMappings map[string]uint16
	apiVersion   string
	baseHostname string
	gatewayIp    string
}

func (d *Docker) SetApiVersion(ver string) {
	d.apiVersion = ver
}
func (d *Docker) SetBaseHostname(hostname string) {
	d.baseHostname = hostname
}
func (d *Docker) SetGatewayIp(gatewayip string) {
	d.gatewayIp = gatewayip
}
func (d *Docker) FetchPortMappings() {
	cli, err := client.NewEnvClient()
	if len(d.apiVersion) > 0 {
		cli, err = client.NewClientWithOpts(client.WithVersion(d.apiVersion))
	}
	if err != nil {
		panic(err)
	}

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
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
func (d *Docker) GetDestinationHostPort(srcHostPort string) (dstHostPort string, err error) {
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
	return "", errors.New(fmt.Sprintf("No destination found for host '%s'", srcHost))
}
func (d *Docker) Handler(w http.ResponseWriter, r *http.Request) {
	dstHostPort, err := d.GetDestinationHostPort(r.Host)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}

	ps := &ProxyServer{}
	if ps.IsWebsocket(r) {
		handler := ps.Websocket(dstHostPort)
		handler.ServeHTTP(w, r)
		return
	}

	handler := &httputil.ReverseProxy{
		Transport: errorHandlingTransport{http.DefaultTransport},
		Director: func(req *http.Request) {
			req.URL.Host = dstHostPort
			req.URL.Scheme = "http"
		},
	}
	handler.ServeHTTP(w, r)
}
