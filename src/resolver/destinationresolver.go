package resolver

import (
	"fmt"
	"os"
	"syscall"
)

type DestinationResolver interface {
	Configure()
	GetName() string
	GetDestinationHostPort(srcHostPort string) (dstHostPort string, err error)
}

func exitWithError(err error) {
	fmt.Printf("%v\n", err)
	os.Exit(1)
}

func gatewayIp() string {
	cli, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
	if err != nil {
		exitWithError(err)
	}
	defer syscall.Close(cli)
	srv, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_ICMP)
	if err != nil {
		exitWithError(err)
	}
	defer syscall.Close(srv)

	ttl := 1
	if err := syscall.SetsockoptInt(cli, syscall.SOL_IP, syscall.IP_TTL, ttl); err != nil {
		exitWithError(err)
	}
	addr := &syscall.SockaddrInet4{Port: 33333, Addr: [4]byte{8, 8, 8, 8}}
	if err := syscall.Sendto(cli, []byte{}, 0, addr); err != nil {
		exitWithError(err)
	}

	_, from, _ := syscall.Recvfrom(srv, []byte{}, 0)
	b := from.(*syscall.SockaddrInet4).Addr
	return fmt.Sprintf("%v.%v.%v.%v", b[0], b[1], b[2], b[3])
}
