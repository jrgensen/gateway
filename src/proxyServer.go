package main

import (
	"./resolver"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
)

type ProxyServer struct {
	destinationResolver  resolver.DestinationResolver
	destinationResolvers map[string]resolver.DestinationResolver
}

func (s *ProxyServer) AddDestinationResolvers(dstRes ...resolver.DestinationResolver) {
	if s.destinationResolvers == nil {
		s.destinationResolvers = make(map[string]resolver.DestinationResolver)
	}
	for _, resolver := range dstRes {
		s.destinationResolvers[resolver.GetName()] = resolver
	}
}
func (s *ProxyServer) SetActiveDestinationResolver(name string) {
	if resolver, ok := s.destinationResolvers[name]; ok {
		resolver.Configure()
		s.destinationResolver = resolver
		fmt.Printf("using destination resolver '%s'\n", resolver.GetName())
		return
	}
	exitWithError(errors.New(fmt.Sprintf("Unknown destination resolver '%s'", name)))
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
	dstHostPort, err := s.destinationResolver.GetDestinationHostPort(r.Host)
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
	handler.ServeHTTP(w, r)
	//loggingRW := &loggingResponseWriter{
	//	ResponseWriter: w,
	//}
	//h.ServeHTTP(loggingRW, r)
	//requestDump, err := httputil.DumpRequest(r, true)
	//handler.ServeHTTP(loggingRW, r)
	//
	//	if err != nil {
	//		fmt.Println(err)
	//	}
	/*if os.Getenv("INSPECT_TRAFFIC") != "" {
		log.Println("#######################################################################")
		log.Println(r.Method, r.URL.Path)
		log.Println(string(requestDump))
		log.Println("Status : ", loggingRW.status, "Header : ", loggingRW.Header(), "Response : ", string(loggingRW.body))
	}*/
}
