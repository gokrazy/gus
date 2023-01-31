package main

import (
	"log"

	"github.com/gokrazy/gus/internal/gusserver"
)

func main() {
	if err := gusserver.Main(); err != nil {
		log.Fatal(err)
	}
}
