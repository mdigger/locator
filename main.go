package main

import "log"

func main() {
	var server = Server{}
	log.Fatal(server.ListenAndServe())
}
