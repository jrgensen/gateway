package main

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
)

type ResolverPrivNet struct {
	proxyOnlyMappedHosts bool
	proxyMappings        map[string]string
	defaultDestination   string
}

func (pn *ResolverPrivNet) SetProxyOnlyMappedHosts(onlyMapped bool) {
	pn.proxyOnlyMappedHosts = onlyMapped
}
func (pn *ResolverPrivNet) SetProxyDefaultHost(defaultHost string) {
	pn.defaultDestination = defaultHost
}
func (pn *ResolverPrivNet) splitHostHostPort(str string) (srcHost string, dstHost string, dstPort int, err error) {
	hhp := strings.Split(str, ":")

	port, _ := strconv.Atoi(hhp[len(hhp)-1])
	if port == 0 {
		port = 80
		hhp = append(hhp, "80")
	}
	if len(hhp) > 3 {
		return "", "", -1, errors.New("Wrong format '%s' expected [srchost:]dsthost[:destport]")
	}
	if len(hhp) == 2 {
		hhp = []string{hhp[0], hhp[0], hhp[1]}
	}
	return hhp[0], hhp[1], port, nil
}

func (pn *ResolverPrivNet) SetProxyMappings(mappings []string) {
	pn.proxyMappings = make(map[string]string)
	for _, hostport := range mappings {
		src, dst, port, err := pn.splitHostHostPort(hostport)
		if err != nil {
			log.Fatal(fmt.Sprintf("Error parsing proxy mapping: %s - %v", hostport, err))
		}
		pn.proxyMappings[src] = fmt.Sprintf("%s:%d", dst, port)
	}
}

func (pn *ResolverPrivNet) GetDestinationHostPort(sourceHost string) (dstHostPort string, err error) {
	srcHost := strings.Split(sourceHost, ".")[0]
	if dstHostPort, ok := pn.proxyMappings[srcHost]; ok {
		return dstHostPort, nil
	}
	if pn.proxyOnlyMappedHosts {
		return "", errors.New(fmt.Sprintf("Only configured gateways allowed ('%s' not found)", srcHost))
	}
	return fmt.Sprintf("%s:%d", srcHost, 80), nil
}
