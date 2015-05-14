package main

import (
	"log"
	"os"
	"path/filepath"
)

func main() {
	go func() {
		var server = new(Server)
		var (
			currentDir = filepath.Dir(os.Args[0])
			cert       = filepath.Join(currentDir, "cert.pem")
			key        = filepath.Join(currentDir, "key.pem")
		)
		log.Println(server.ListenAndServeTLS(cert, key))
	}()
	var server = new(Server)
	log.Fatal(server.ListenAndServe())
}
