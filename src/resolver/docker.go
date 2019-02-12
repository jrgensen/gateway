package resolver

import (
	"errors"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
	"github.com/namsral/flag"
	"golang.org/x/net/context"

	"io"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
)

type Docker struct {
	proxyOnlyMappedHosts bool
	proxyMappings        map[string]string
	portMappings         map[string]uint16
	baseHostname         string
	gatewayIp            string
	client               *client.Client
	info                 types.Info
}

func (d *Docker) Configure() {
	flag.BoolVar(&d.proxyOnlyMappedHosts, "proxy-only-mapped-hosts", false, "Only hosts specified in proxy mapping will be proxied")
	flag.StringVar(&d.baseHostname, "base-hostname", "", "Proxy key is first subdomaine to base host")
	flag.StringVar(&d.gatewayIp, "gateway-ip", gatewayIp(), "Specify gateway ip")

	var mappings string
	flag.StringVar(&mappings, "proxy-mappings", "", "Manually specify mappings")
	flag.Parse()

	d.proxyMappings = d.parseProxyMappings(mappings)

	var err error
	d.client, err = client.NewClientWithOpts(client.WithVersion("1.30")) //1.18
	if err != nil {
		panic(err)
	}

	d.info, _ = d.client.Info(context.Background())
	fmt.Printf("Swarm mode: %+v\n", d.info.Swarm.ControlAvailable)

	d.fetchPortMappings()
	go d.listenEvents()
}

func (d *Docker) GetName() string {
	return "docker"
}

func (d *Docker) parseProxyMappings(mappings string) map[string]string {
	proxyMap := make(map[string]string)
	for _, mapping := range strings.Fields(mappings) {
		hhp := strings.Split(mapping, ":")

		if _, err := strconv.Atoi(hhp[len(hhp)-1]); err != nil {
			hhp = append(hhp, "0")
		}
		if len(hhp) > 3 {
			log.Printf("Wrong mapping format '%s' expected [srchost:]dsthost[:destport]", mapping)
			continue
		}
		if len(hhp) == 2 {
			hhp = []string{hhp[0], hhp[0], hhp[1]}
		}
		proxyMap[hhp[0]] = fmt.Sprintf("%s:%s", hhp[1], hhp[2])
	}
	return proxyMap
}

func (d *Docker) listenEvents() {
	messages, errs := d.client.Events(context.Background(), types.EventsOptions{})
	fmt.Println("Listening for docker events:")
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
			fmt.Printf("Refreshing port mapping [%s]: ", e.Type)
			d.fetchPortMappings()
		}
	}
}

func (d *Docker) fetchPortMappings() {
	portMappings := make(map[string]uint16)

	containers, err := d.client.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		log.Println(err)
		return
	}
	for _, container := range containers {
		for _, port := range container.Ports {
			if port.PrivatePort == 80 && port.Type == "tcp" && port.PublicPort > 0 && port.PrivatePort != port.PublicPort {
				for _, name := range container.Names {
					for i := len(name); i > -1; i = strings.LastIndex(name, "_") {
						name = name[0:i]
						portMappings[name[1:]] = port.PublicPort
					}
				}
			}
		}
	}

	services, err := d.client.ServiceList(context.Background(), types.ServiceListOptions{})
	if err != nil {
		log.Println(err)
		return
	}
	for _, service := range services {
		for _, port := range service.Endpoint.Ports {
			if port.Protocol == "tcp" && port.TargetPort == 80 {
				name := service.Spec.Name
				for i := len(name); i > -1; i = strings.LastIndex(name, "_") {
					name = name[0:i]
					portMappings[name[0:]] = uint16(port.PublishedPort)
				}
			}
		}
	}
	d.portMappings = portMappings
	fmt.Println(d.portMappings)
}

func (d *Docker) GetDestinationHostPort(srcHostPort string) (dstHostPort string, err error) {
	srcHost := strings.Split(srcHostPort, ":")[0]
	fmt.Printf("Key: [%s]\n", srcHost)
	if dstHostPort, ok := d.proxyMappings[srcHost]; ok {
		dstHost, dstPort, _ := net.SplitHostPort(dstHostPort)
		if dstPort != "0" {
			return dstHostPort, nil
		}
		if dstPort, ok := d.portMappings[dstHost]; ok {
			return fmt.Sprintf("%s:%d", dstHost, dstPort), nil
		}
		return "", errors.New(fmt.Sprintf("No destination Found for host '%s' (%s)", srcHost, dstHost))
	}

	srcHost = regexp.MustCompile("([^\\.]+)\\.local\\.").FindStringSubmatch(srcHost)[1]
	fmt.Printf("Key: [%s]\n", srcHost)
	if dstHostPort, ok := d.proxyMappings[srcHost]; ok {
		dstHost, dstPort, _ := net.SplitHostPort(dstHostPort)
		if dstPort != "0" {
			return dstHostPort, nil
		}
		if dstPort, ok := d.portMappings[dstHost]; ok {
			return fmt.Sprintf("%s:%d", dstHost, dstPort), nil
		}
		return "", errors.New(fmt.Sprintf("No destination Found for host '%s' (%s)", srcHost, dstHost))
	}

	if d.proxyOnlyMappedHosts {
		return "", errors.New(fmt.Sprintf("Only configured gateways allowed ('%s' not found)", srcHost))
	}

	dstHost := srcHost
	if dstPort, ok := d.portMappings[dstHost]; ok {
		return fmt.Sprintf("%s:%d", dstHost, dstPort), nil
	}
	return "", errors.New(fmt.Sprintf("No destination Found for host '%s' (%s)", srcHost, dstHost))
}
