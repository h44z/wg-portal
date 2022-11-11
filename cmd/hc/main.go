// source taken from https://git.prolicht.digital/golib/healthcheck/-/blob/master/cmd/hc/main.go

package main

import (
	"net/http"
	"os"
	"time"
)

// main checks the given URL, if the response is not 200, it will return with exit code 1
// on success, exit code 0 will be returned
func main() {
	os.Exit(checkWebEndpointFromArgs())
}

func checkWebEndpointFromArgs() int {
	if len(os.Args) < 2 {
		return 1
	}
	if status := checkWebEndpoint(os.Args[1]); !status {
		return 1
	}
	return 0
}

func checkWebEndpoint(url string) bool {
	client := &http.Client{
		Timeout: time.Second * 2,
	}
	if resp, err := client.Get(url); err != nil || resp.StatusCode < 200 || resp.StatusCode > 299 {
		return false
	}
	return true
}
