package app

import (
	"fmt"
	"net/http"
)

func Run(addr string, r http.Handler) error {
	fmt.Println("running server on", addr)
	return http.ListenAndServe(addr, r)
}
