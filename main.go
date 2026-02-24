package main

import "net/http"


func main() {
	handler := http.NewServeMux()
	server := http.Server{
		Handler: handler,
		Addr: ":8080",
	}

	server.ListenAndServe()
}
