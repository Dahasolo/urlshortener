package main

import (
	"log"

	"github.com/Dahasolo/urlshortener/internal/app"
)

func main() {
	app, err := app.NewAppFromFlags()
	if err != nil {
		log.Fatal("app initialization failed:", err)
	}

	if err := app.Run(); err != nil {
		log.Fatal("server failed:", err)
	}
}
