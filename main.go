package main

import (
	"flag"
	"log"
)

func main() {
	// log.SetFlags(log.Lshortfile)

	var (
		port = flag.String("port", ":9000", "server port")
	)
	flag.Parse()

	server := Server{
		Addr: *port,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatal("Server start error:", err)
	}
}
