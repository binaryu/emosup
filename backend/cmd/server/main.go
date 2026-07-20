package main

import (
	"log"
	"os"

	"emosup/backend/internal/app"
)

// Set by release builds: -ldflags "-X main.version=v1.0.0"
var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "-version", "--version", "-v":
			log.Printf("emosup %s", version)
			return
		}
	}

	application, err := app.New()
	if err != nil {
		log.Fatalf("bootstrap app failed: %v", err)
	}

	log.Printf("emosup %s starting", version)
	if err := application.Run(); err != nil {
		log.Fatalf("run server failed: %v", err)
	}
}
