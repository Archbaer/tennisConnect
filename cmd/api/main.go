package main

import (
	"log"
	"os"

	"tennisconnect/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
