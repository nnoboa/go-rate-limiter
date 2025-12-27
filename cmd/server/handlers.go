package main

import (
	"fmt"
	"net/http"
)

// A simple API endpoint.
func HelloWorldHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello, World!")
}
