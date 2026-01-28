package app

import (
	"net/http"
)

func Run(addr string, r http.Handler) error {
	return http.ListenAndServe(addr, r)
}
