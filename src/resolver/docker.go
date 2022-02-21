package resolver

import (
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/namsral/flag"
	"golang.org/x/net/context"
)

type Stack struct {
	services    []swarm.Service
	healthLabel string
}

func (s *Stack) CreatedAt() time.Time {
	createdAt := time.Now()
	for _, service := range s.services {
		if createdAt.After(service.CreatedAt) {
			createdAt = service.CreatedAt
		}
	}
	return createdAt
}
func (s *Stack) Healthy() bool {
	for _, service := range s.services {
		if value, found := service.Spec.Labels[s.healthLabel]; !found || value != "true" {
			return false
		}
	}
	return true
}
func (s *Stack) Namespace(label string) string {
	namespace := ""
	for _, service := range s.services {
		if name, found := service.Spec.Labels[label]; found {
			return name
		}
		namespace = service.Spec.Labels["com.docker.stack.namespace"]
	}
	return namespace
}

func (s *Stack) Ports() map[uint32]uint32 {
	ports := map[uint32]uint32{}
	for _, service := range s.services {
		for _, port := range service.Endpoint.Ports {
			if port.Protocol != "tcp" {
				continue
			}
			ports[port.TargetPort] = port.PublishedPort
		}
	}
	return ports
}

type Deployment struct {
	stacks map[string]Stack
}

func (d *Deployment) ActiveStack() (activeStack *Stack) {
	for _, stack := range d.stacks {
		if !stack.Healthy() {
			continue
		}
		if activeStack == nil {
			activeStack = &stack
			continue
		}
		if activeStack.CreatedAt().Before(stack.CreatedAt()) {
			activeStack = &stack
		}
	}
	return
}
func (d *Deployment) NewestStack() (newestStack *Stack) {
	for _, stack := range d.stacks {
		if newestStack == nil {
			newestStack = &stack
			continue
		}
		if newestStack.CreatedAt().Before(stack.CreatedAt()) {
			newestStack = &stack
		}
	}
	return
}

type Swarm struct {
	deployments     map[string]Deployment
	deploymentLabel string
	healthLabel     string
}

func (s *Swarm) AddServices(services []swarm.Service) {
	stacks := map[string]Stack{}
	for _, service := range services {
		stackName := service.Spec.Labels["com.docker.stack.namespace"]
		stack := stacks[stackName]
		stack.services = append(stack.services, service)
		stacks[stackName] = stack
	}
	s.deployments = map[string]Deployment{}
	for name, stack := range stacks {
		stack.healthLabel = s.healthLabel
		namespace := stack.Namespace(s.deploymentLabel)
		deployment := s.deployments[namespace]
		if deployment.stacks == nil {
			deployment.stacks = map[string]Stack{}
		}
		deployment.stacks[name] = stack
		s.deployments[namespace] = deployment
	}
}
func (s *Swarm) Ports() map[string]uint16 {
	ports := map[string]uint16{}
	for name, deployment := range s.deployments {
		stack := deployment.ActiveStack()
		if stack == nil {
			stack = deployment.NewestStack()
		}
		for targetPort, publishedPort := range stack.Ports() {
			ports[fmt.Sprintf("%s:%d", name, targetPort)] = uint16(publishedPort)
		}
	}
	return ports
}

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

	stackLabel  string
	healthLabel string
	swarm       Swarm
}

func (d *Docker) Configure() {
	flag.BoolVar(&d.proxyOnlyMappedHosts, "proxy-only-mapped-hosts", false, "Only hosts specified in proxy mapping will be proxied")
	flag.StringVar(&d.baseHostname, "base-hostname", "", "Proxy key is first subdomaine to base host")
	flag.StringVar(&d.gatewayIp, "gateway-ip", gatewayIp(), "Specify gateway ip")
	flag.StringVar(&d.stackSearchString, "stack-search-string", "([^\\.]+)\\.(local|dev|build|test|stage|preprod|prod)\\.", "How to identify a stack from hostname")
	flag.StringVar(&d.stackLabel, "docker-stack-label", "", "Name of label defining the stack")
	flag.StringVar(&d.healthLabel, "docker-health-label", "", "Name of label specifing the service health")

	var mappings string
	flag.StringVar(&mappings, "proxy-mappings", "", "Manually specify mappings")
	flag.Parse()

	d.proxyMappings, d.innerPorts = d.parseProxyMappings(mappings)

	var err error
	d.client, err = client.NewClientWithOpts(client.WithVersion("1.30")) //1.18
	if err != nil {
		panic(err)
	}
	d.swarm = Swarm{deploymentLabel: d.stackLabel, healthLabel: d.healthLabel}

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
	filters := filters.NewArgs()
	// Include specific event types
	filters.Add("type", events.ContainerEventType)
	filters.Add("type", events.NetworkEventType)
	filters.Add("type", events.ServiceEventType)

	messages, errs := d.client.Events(context.Background(), types.EventsOptions{Filters: filters})
	fmt.Println("Listening for docker events:")
	for {
		select {
		case err := <-errs:
			if err != nil && err != io.EOF {
				log.Fatal(err)
			}
			return
		case e := <-messages:
			// Exclude specific actions
			if e.Action == "health_status" {
				break
			}
			if strings.HasPrefix(e.Action, "exec_") {
				break
			}

			fmt.Printf("Refreshing port mapping [%s] %s: ", e.Type, e.Action)
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
				if container.Labels["gateway.stack.name"] != "" {
					portMappings[fmt.Sprintf("%s:%d", container.Labels["gateway.stack.name"], port.PrivatePort)] = port.PublicPort
					continue
				}
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
	services, err := d.client.ServiceList(context.Background(), types.ServiceListOptions{})
	if err != nil {
		log.Println(err)
		return map[string]uint16{}
	}
	d.swarm.AddServices(services)
	return d.swarm.Ports()
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
