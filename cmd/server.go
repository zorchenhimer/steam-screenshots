package main

import (
	"os"
	"fmt"

	ss "github.com/zorchenhimer/steam-screenshots"

	"github.com/alexflint/go-arg"
)

type Arguments struct {
	SettingsFile string `arg:"-c,--config" default:"settings.json"`
}

func main() {
	args := &Arguments{}
	arg.MustParse(args)

	server, err := ss.NewServer(args.SettingsFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	err = server.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
