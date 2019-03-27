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
	//"net"
	"regexp"
	"strconv"
	"strings"
)

type Docker struct {
	proxyOnlyMappedHosts bool
	proxyMappings        map[string]string
	portMappings         map[string]uint16
	innerPorts           map[string]uint16
	stackSearchString    string
	baseHostname         string
	gatewayIp            string
	client               *client.Client
	info                 types.Info
}

func (d *Docker) Configure() {
	flag.BoolVar(&d.proxyOnlyMappedHosts, "proxy-only-mapped-hosts", false, "Only hosts specified in proxy mapping will be proxied")
	flag.StringVar(&d.baseHostname, "base-hostname", "", "Proxy key is first subdomaine to base host")
	flag.StringVar(&d.gatewayIp, "gateway-ip", gatewayIp(), "Specify gateway ip")
	flag.StringVar(&d.stackSearchString, "stack-search-string", "([^\\.]+)\\.(local|dev|build|test|stage|preprod|prod)\\.", "How to identify a stack from hostname")

	var mappings string
	flag.StringVar(&mappings, "proxy-mappings", "", "Manually specify mappings")
	flag.Parse()

	d.proxyMappings, d.innerPorts = d.parseProxyMappings(mappings)

	var err error
	d.client, err = client.NewClientWithOpts(client.WithVersion("1.30")) //1.18
	if err != nil {
		panic(err)
	}

	d.fetchPorts()
	go d.listenEvents()
	log.Printf("%#v\n", d)
}

func (d *Docker) GetName() string {
	return "docker"
}

func (d *Docker) parseProxyMappings(mappings string) (map[string]string, map[string]uint16) {
	proxyMap := make(map[string]string)
	innerPorts := make(map[string]uint16)
	for _, mapping := range strings.Fields(mappings) {
		hhp := strings.Split(mapping, ":")

		if _, err := strconv.Atoi(hhp[len(hhp)-1]); err != nil {
			hhp = append(hhp, "80")
		}
		if len(hhp) > 3 {
			log.Printf("Wrong mapping format '%s' expected [srchost:]dsthost[:destport]", mapping)
			continue
		}
		if len(hhp) == 2 {
			hhp = []string{hhp[0], hhp[0], hhp[1]}
		}
		proxyMap[hhp[0]] = fmt.Sprintf("%s:%s", hhp[1], hhp[2])
		innerPort, _ := strconv.Atoi(hhp[2])
		innerPorts[hhp[1]] = uint16(innerPort)
	}
	return proxyMap, innerPorts
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
			d.fetchPorts()
		}
	}
}

func (d *Docker) fetchContainerPorts() map[string]uint16 {
	portMappings := make(map[string]uint16)

	containers, err := d.client.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		log.Println(err)
		return portMappings
	}
	for _, container := range containers {
		for _, port := range container.Ports {
			if port.Type == "tcp" && port.PublicPort > 0 {
				for _, name := range container.Names {
					for i := len(name); i > -1; i = strings.LastIndex(name, "_") {
						name = name[0:i]
						portMappings[fmt.Sprintf("%s:%d", name[1:], port.PrivatePort)] = port.PublicPort
					}
				}
			}
		}
	}
	return portMappings
}

func (d *Docker) fetchServicePorts() map[string]uint16 {
	portMappings := make(map[string]uint16)

	services, err := d.client.ServiceList(context.Background(), types.ServiceListOptions{})
	if err != nil {
		log.Println(err)
		return portMappings
	}
	for _, service := range services {
		for _, port := range service.Endpoint.Ports {
			name := service.Spec.Name
			for i := len(name); i > -1; i = strings.LastIndex(name, "_") {
				name = name[0:i]
				if port.Protocol == "tcp" {
					portMappings[fmt.Sprintf("%s:%d", name, port.TargetPort)] = uint16(port.PublishedPort)
				}
			}
		}
	}
	return portMappings
}

func (d *Docker) fetchPorts() {
	info, _ := d.client.Info(context.Background())
	fmt.Printf("Swarm mode: %+v\n", info.Swarm.ControlAvailable)

	ports := d.fetchContainerPorts()
	if info.Swarm.ControlAvailable {
		for k, v := range d.fetchServicePorts() {
			ports[k] = v
		}
	}

	d.portMappings = ports
	fmt.Println(d.portMappings)
}

func (d *Docker) GetDestinationHostPort(srcHostPort string) (dstHostPort string, err error) {
	srcHost := strings.Split(srcHostPort, ":")[0]
	dstHost := d.gatewayIp
	fmt.Printf("Key: [%s]\n", srcHost)

	if dstHostPort, ok := d.proxyMappings[srcHost]; ok {
		if dstPort, ok := d.portMappings[dstHostPort]; ok {
			return fmt.Sprintf("%s:%d", dstHost, dstPort), nil
		}
		return "", errors.New(fmt.Sprintf("No destination found for host '%s' (%s)", srcHost, dstHostPort))
	}

	srcHostLevels := regexp.MustCompile(d.stackSearchString).FindStringSubmatch(srcHost)
	if len(srcHostLevels) > 1 {
		srcHost = srcHostLevels[1]
		if dstHostPort, ok := d.proxyMappings[srcHost]; ok {
			if dstPort, ok := d.portMappings[dstHostPort]; ok {
				return fmt.Sprintf("%s:%d", dstHost, dstPort), nil
			}
			return "", errors.New(fmt.Sprintf("No destination found for stack name '%s' (%s)", srcHost, dstHost))
		}
	}

	if d.proxyOnlyMappedHosts {
		return "", errors.New(fmt.Sprintf("Only configured gateways allowed ('%s' not found)", srcHost))
	}

	dstHostPort = fmt.Sprintf("%s:%d", srcHost, 80)
	if dstPort, ok := d.portMappings[dstHostPort]; ok {
		return fmt.Sprintf("%s:%d", dstHost, dstPort), nil
	}
	return "", errors.New(fmt.Sprintf("No destination, exhausted all methods '%s' (%s)", srcHost, dstHostPort))
}
