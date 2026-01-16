package app

import (
	"log"
	"net/http"
)

func Run(addr string, r http.Handler) error {
	log.Println("running server on", addr)
	return http.ListenAndServe(addr, r)
}
