package main

import (
	"log"
	"os"

	"emosup/backend/internal/app"
	"emosup/backend/internal/version"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "-version", "--version", "-v":
			log.Printf("emosup %s", version.Version)
			return
		}
	}

	application, err := app.New()
	if err != nil {
		log.Fatalf("bootstrap app failed: %v", err)
	}

	log.Printf("emosup %s starting", version.Version)
	if err := application.Run(); err != nil {
		log.Fatalf("run server failed: %v", err)
	}
}
