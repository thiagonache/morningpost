package main

import (
	"log"

	"github.com/thiagonache/morningpost"
)

func main() {
	if err := morningpost.Run(); err != nil {
		log.Fatal(err)
	}
}
