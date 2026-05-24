package main

import (
	"log"

	"harish.com/keenbase/keenbase"
)

func main() {
	sb := keenbase.New()
	if err := sb.Start(); err != nil {
		log.Fatal(err)
	}
}
