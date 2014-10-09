package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/ninjasphere/go-ninja/logger"
)

var log = logger.GetLogger("driver-go-flowerpower")

func main() {

	_, err := NewFlowerPowerDriver()

	if err != nil {
		log.Fatalf("Failed to create Flower Power driver: %s", err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	// Block until a signal is received.
	s := <-c
	fmt.Println("Got signal:", s)

}
