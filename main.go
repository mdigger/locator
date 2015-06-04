package main

import (
	"log"
	"os"
	"path/filepath"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	var list = NewList() // инициализируем список соединений
	go func() {
		var server = NewServer(list)
		var (
			currentDir = filepath.Dir(os.Args[0])
			cert       = filepath.Join(currentDir, "cert.pem")
			key        = filepath.Join(currentDir, "key.pem")
		)
		log.Println(server.ListenAndServeTLS(cert, key))
	}()
	var server = NewServer(list)
	log.Fatal(server.ListenAndServe())
}
