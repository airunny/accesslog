package main

import (
	"encoding/json"
	"net/http"

	"github.com/liyanbing/accesslog"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/abc", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-type", "application/json")
		resp := map[string]string{
			"user":     "Peter",
			"position": "manager",
		}
		json.NewEncoder(w).Encode(resp)
	})

	http.ListenAndServe(":8080", accesslog.Handler(mux, accesslog.WithFileName("./access/example.log")))
}
