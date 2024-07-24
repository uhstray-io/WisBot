package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	go StartBot()
	go WebServer()

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
