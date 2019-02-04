package main

import (
	"fmt"
	"log"
	//"net"
	"net/http"
	"os"

	"github.com/namsral/flag"

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
	)
	flag.Int64Var(&portProxy, "port", 80, "Port gateway proxy will be listening on")
	flag.Int64Var(&portInspector, "port-inspector", 0, "Port gateway inspector will be listening on")
	flag.StringVar(&resolverName, "destination-resolver", "subnet", "The destination resolver to use (subnet, docker)")

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
	http.HandleFunc("/", handler)

	fmt.Println("gatway proxy listening on port", portProxy)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", portProxy), nil))
}

func attic() {
	//resolver.GetHostIp()
	//hostname := os.Getenv("HOSTNAME")
	//httpPort, _ := strconv.ParseInt(getEnv("HTTP_PORT", "80"), 10, 0)
	//port := flag.Int64("port", httpPort, "Listening on port")
	//defaultHostEnv := getEnv("PROXY_DEFAULT_HOST", "")
	//defaultHost := flag.String("default", defaultHostEnv, "The service being proxied if base address is used.")

	//ihost := flag.String("host", "localhost", "inspection host")
	//hostip := flag.String("hostip", "localhost", "ip address of gateway")

	/*
		if os.Getenv("DESTINATION_RESOLVER") == "docker" {
			docker := resolver.NewDockerResolver()
			docker.SetBaseHostname(hostname)
			docker.SetGatewayIp(*hostip)
			docker.FetchPortMappings()
			ps.SetDestinationResolver(docker)
		} else {
			priv := &resolver.Subnet{}
			priv.SetProxyDefaultHost(*defaultHost)
			ps.SetDestinationResolver(priv)
		}
	*/
	//http.HandleFunc(fmt.Sprintf("%s/", *ihost), wrapHandler(hello))
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
