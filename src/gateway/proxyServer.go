package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"
)

type ProxyServer struct {
	proxyOnlyMappedHosts bool
	proxyMappings        map[string]string
	defaultDestination   string
}

func (s *ProxyServer) SetProxyOnlyMappedHosts(onlyMapped bool) {
	s.proxyOnlyMappedHosts = onlyMapped
}
func (s *ProxyServer) SetProxyDefaultHost(defaultHost string) {
	s.defaultDestination = defaultHost
}
func (s *ProxyServer) splitHostHostPort(str string) (srcHost string, dstHost string, dstPort int, err error) {
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

func (s *ProxyServer) SetProxyMappings(mappings []string) {
	s.proxyMappings = make(map[string]string)
	for _, hostport := range mappings {
		src, dst, port, err := s.splitHostHostPort(hostport)
		if err != nil {
			log.Fatal(fmt.Sprintf("Error parsing proxy mapping: %s - %v", hostport, err))
		}
		s.proxyMappings[src] = fmt.Sprintf("%s:%d", dst, port)
	}
}

func (s *ProxyServer) GetDestinationHostPort(srcHost string) (dstHostPort string, err error) {
	if dstHostPort, ok := s.proxyMappings[srcHost]; ok {
		return dstHostPort, nil
	}
	if s.proxyOnlyMappedHosts {
		return "", errors.New(fmt.Sprintf("Only configured gateways allowed ('%s' not found)", srcHost))
	}
	return fmt.Sprintf("%s:%d", srcHost, 80), nil
}
func (s *ProxyServer) Websocket(target string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d, err := net.Dial("tcp", target)
		if err != nil {
			http.Error(w, "Error contacting backend server.", 500)
			log.Printf("Error dialing websocket backend %s: %v", target, err)
			return
		}
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "Not a hijacker?", 500)
			return
		}
		nc, _, err := hj.Hijack()
		if err != nil {
			log.Printf("Hijack error: %v", err)
			return
		}
		defer nc.Close()
		defer d.Close()

		err = r.Write(d)
		if err != nil {
			log.Printf("Error copying request to target: %v", err)
			return
		}

		errc := make(chan error, 2)
		cp := func(dst io.Writer, src io.Reader) {
			_, err := io.Copy(dst, src)
			errc <- err
		}
		go cp(d, nc)
		go cp(nc, d)
		<-errc
	})
}

func (s *ProxyServer) IsWebsocket(req *http.Request) bool {
	// if this is not an upgrade request it's not a websocket
	if len(req.Header["Connection"]) == 0 || strings.ToLower(req.Header["Connection"][0]) != "upgrade" {
		return false
	}
	if len(req.Header["Upgrade"]) == 0 {
		return false
	}

	return (strings.ToLower(req.Header["Upgrade"][0]) == "websocket")
}

func (s *ProxyServer) Handler(w http.ResponseWriter, r *http.Request) {
	srcHost := strings.Split(r.Host, ".")[0]
	dstHostPort, err := s.GetDestinationHostPort(srcHost)
	if err != nil {

		http.Error(w, err.Error(), 502)
		return
		//fmt.Println(err)
	}

	if s.IsWebsocket(r) {
		handler := s.Websocket(dstHostPort)
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
	loggingRW := &loggingResponseWriter{
		ResponseWriter: w,
	}
	//h.ServeHTTP(loggingRW, r)
	requestDump, err := httputil.DumpRequest(r, true)
	handler.ServeHTTP(loggingRW, r)

	if err != nil {
		fmt.Println(err)
	}
	if os.Getenv("INSPECT_TRAFFIC") != "" {
		log.Println("#######################################################################")
		log.Println(r.Method, r.URL.Path)
		log.Println(string(requestDump))
		log.Println("Status : ", loggingRW.status, "Header : ", loggingRW.Header(), "Response : ", string(loggingRW.body))
	}
}
