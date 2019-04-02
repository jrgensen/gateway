package resolver

import (
	"errors"
	"fmt"
	"github.com/namsral/flag"
	"log"
	"strconv"
	"strings"
)

type Subnet struct {
	proxyOnlyMappedHosts bool
	proxyMappings        map[string]string
}

func (s *Subnet) Configure() {
	flag.BoolVar(&s.proxyOnlyMappedHosts, "proxy-only-mapped-hosts", false, "Only hosts specified in proxy mapping will be proxied")

	var mappings string
	flag.StringVar(&mappings, "proxy-mappings", "", "Manually specify mappings")
	flag.Parse()

	s.proxyMappings = s.parseProxyMappings(mappings)
	//resolver.SetProxyMappings(strings.Fields(mappings))
}
func (s *Subnet) GetName() string {
	return "subnet"
}

func (s *Subnet) parseProxyMappings(mappings string) map[string]string {
	proxyMap := make(map[string]string)
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
	}
	return proxyMap
}

func (s *Subnet) GetDestinationHostPort(sourceHostPort string) (dstHostPort string, err error) {
	sourceHost := strings.Split(sourceHostPort, ":")[0]

	// Full host matching
	if dstHostPort, ok := s.proxyMappings[sourceHost]; ok {
		return dstHostPort, nil
	}

	// First part of host matching
	srcHost := strings.Split(sourceHost, ".")[0]
	if dstHostPort, ok := s.proxyMappings[srcHost]; ok {
		return dstHostPort, nil
	}

	// Arbitrary number of host parts matching
	for src, dst := range s.proxyMappings {
		if strings.HasPrefix(sourceHost, src+".") {
			return dst, nil
		}
	}

	// Don't assume fallback, if we only proxy mapped hosts
	if s.proxyOnlyMappedHosts {
		return "", errors.New(fmt.Sprintf("Only configured gateways allowed ('%s' not found)", srcHost))
	}

	// Fallback, assume first part host exists
	return fmt.Sprintf("%s:%d", srcHost, 80), nil
}
