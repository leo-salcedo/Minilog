package main

import (
	"fmt"
	"net/http"

	"minilog/internal/api"
	"minilog/internal/logstore"
)

func main() {
	store := logstore.NewStore()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "minilog running")
	})
	http.Handle("/logs", api.NewLogsHandler(store))

	fmt.Println("Server running on :8080")
	http.ListenAndServe(":8080", nil)
}
