package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type loggingResponseWriter struct {
	status int
	body   []byte
	http.ResponseWriter
}

func (w *loggingResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *loggingResponseWriter) Write(body []byte) (int, error) {
	w.body = body
	return w.ResponseWriter.Write(body)
}

func main() {
	b, _ := strconv.ParseBool(os.Getenv("PROXY_ONLY_MAPPED_HOSTS"))
	defaultHostEnv := getEnv("PROXY_DEFAULT_HOST", "")
	dockerApiVersion := os.Getenv("DOCKER_API_VERSION")
	hostname := os.Getenv("HOSTNAME")
	httpPort, _ := strconv.ParseInt(getEnv("HTTP_PORT", "80"), 10, 0)

	port := flag.Int64("port", httpPort, "Listening on port")
	onlyMapped := flag.Bool("only-mapped", b, "Only hosts specified in proxy mapping will be proxied")
	defaultHost := flag.String("default", defaultHostEnv, "The service being proxied if base address is used.")
	ihost := flag.String("host", "localhost", "inspection host")
	hostip := flag.String("hostip", "localhost", "ip address of gateway")
	flag.Parse()

	http.HandleFunc(fmt.Sprintf("%s/", *ihost), hello)

	dpp, _ := strconv.ParseBool(os.Getenv("DOCKER_PORT_PROXY"))
	if dpp {
		docker := &Docker{}
		docker.SetApiVersion(dockerApiVersion)
		docker.SetBaseHostname(hostname)
		docker.SetGatewayIp(*hostip)
		docker.FetchPortMappings()
		http.HandleFunc("/", docker.Handler)
	} else {
		ps := &ProxyServer{}
		ps.SetProxyMappings(strings.Fields(os.Getenv("PROXY_MAPPINGS")))
		ps.SetProxyOnlyMappedHosts(*onlyMapped)
		ps.SetProxyDefaultHost(*defaultHost)
		http.HandleFunc("/", ps.Handler)
	}

	fmt.Println("starting proxy on port", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func hello(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello astaxie!") // send data to client side
}

//type ResponseLogger struct{}
//func (r *ResponseLogger) Write(b []byte) (int, error) {
//  log.Print(string(b)) // log it out
//  return r.w.Write(b) // pass it to the original ResponseWriter
//}
