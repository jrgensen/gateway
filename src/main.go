package main

import (
	"fmt"
	"log"
	//"net"
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/namsral/flag"
	"golang.org/x/crypto/acme/autocert"

	"./resolver"
)

func exitWithError(err error) {
	fmt.Printf("%v\n", err)
	os.Exit(1)
}
func main() {
	var (
		portProxy     int64
		portInspector int64
		resolverName  string
		https         bool
	)
	HOSTS := make(map[string]string, 0)
	for _, mapping := range strings.Fields(getEnv("PROXY_MAPPINGS", "")) {
		hhp := strings.Split(mapping, ":")
		HOSTS[hhp[0]] = mapping
	}
	flag.Int64Var(&portProxy, "port", 80, "Port gateway proxy will be listening on")
	flag.Int64Var(&portInspector, "port-inspector", 0, "Port gateway inspector will be listening on")
	flag.StringVar(&resolverName, "destination-resolver", "subnet", "The destination resolver to use (subnet, docker)")
	flag.BoolVar(&https, "https", false, "Redirect all mapped hosts to https")

	ps := &ProxyServer{}
	ps.AddDestinationResolvers(
		&resolver.Subnet{},
		&resolver.Docker{},
	)

	flag.Parse()
	ps.SetActiveDestinationResolver(resolverName)

	handler := ps.Handler
	if portInspector != 0 {
		handler = wrapHandler(handler, portInspector)
	}
	if https {
		m := autocert.Manager{
			Cache:  autocert.DirCache("/app/certs"),
			Prompt: autocert.AcceptTOS,
			HostPolicy: func(ctx context.Context, host string) error {
				if _, ok := HOSTS[host]; ok {
					return nil
				}
				return errors.New("Unkown host(" + host + ")")
			},
		}
		s := &http.Server{
			Addr:      ":https",
			TLSConfig: &tls.Config{GetCertificate: m.GetCertificate},
			Handler:   http.HandlerFunc(handler),
		}
		go (func() {
			log.Fatal(s.ListenAndServeTLS("", ""))
		})()
		handler = m.HTTPHandler(nil).ServeHTTP
	}
	http.HandleFunc("/", handler)

	fmt.Println("gatway proxy listening on port", portProxy)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", portProxy), nil))
}
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func wrapHandler(h http.HandlerFunc, port int64) http.HandlerFunc {
	fmt.Println("gatway inspector listening on port", port)
	return func(w http.ResponseWriter, r *http.Request) {
		//if !currentUser(r).IsAdmin {
		//    http.NotFound(w, r)
		//    return
		//}
		h(w, r)
	}
}
func hello(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello astaxie!") // send data to client side
}

//type ResponseLogger struct{}
//func (r *ResponseLogger) Write(b []byte) (int, error) {
//  log.Print(string(b)) // log it out
//  return r.w.Write(b) // pass it to the original ResponseWriter
//}
