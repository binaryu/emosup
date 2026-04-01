package main

import (
	"log"

	"emosup/backend/internal/app"
)

func main() {
	application, err := app.New()
	if err != nil {
		log.Fatalf("bootstrap app failed: %v", err)
	}

	if err := application.Run(); err != nil {
		log.Fatalf("run server failed: %v", err)
	}
}
