package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
	_ "modernc.org/sqlite"
)

func main() {
	db, err := initDatabase("database.db")
	PanicOnError(err)
	defer db.Close()

	go StartBot()
	go WebServer()

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
