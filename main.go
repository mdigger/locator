package main

import "log"

func main() {
	// var (
	// 	useTLS = flag.Bool("TLS", false, "use TLS for connection")
	// )
	// flag.Parse()

	var server = new(Server)
	// if *useTLS {
	// 	log.Fatal(server.ListenAndServeTLS())
	// } else {
	log.Fatal(server.ListenAndServe())
	// }
}
