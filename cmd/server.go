package main

import (
	"os"

	ss "github.com/zorchenhimer/steam-screenshots"
)

func main() {
	server := ss.Server{}

	if len(os.Args) > 1 {
		server.SettingsFile = os.Args[1]
	}

	server.Run()
}
