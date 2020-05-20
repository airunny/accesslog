# accesslog

## 安装 
go get github.com/liyanbing/accesslog

## 使用 
```go
package main

import (
	"encoding/json"
    "net/http"

    "github.com/liyanbing/accesslog"
)

func main() {
	conf := &accesslog.Conf{
		Filename:     "./access/example.log", 
		RequestBody:  true, 
		ResponseBody: true, 
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/abc", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-type", "application/json")
		resp := map[string]string{
			"user":     "Peter",
			"position": "manager",
		}
		json.NewEncoder(w).Encode(resp)
	})

	http.ListenAndServe(":8080", accesslog.Handler(conf, mux))

    accesslog.Flush()
}
```
