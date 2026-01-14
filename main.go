package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hello World!")
	})

	fmt.Println("Server listening on port 8080...")
	if err := http.ListenAndServe("0.0.0.0:8080", nil); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}
