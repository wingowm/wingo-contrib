package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/BurntSushi/ty/fun"
)

var (
	command  string // one of the keys in `commands`
	commands = map[string]struct {
		do   func()
		desc string
	}{
		"install": {install, "add scripts from wingo-contrib"},
		"upgrade": {upgrade, "update a script"},
		"search":  {search, "find scripts by searching descriptions"},
		"info":    {info, "show information about script"},
		"list":    {list, "lists all installed scripts from wingo-contrib"},
	}
)

func main() {
	log.SetFlags(0)
	if len(os.Args) == 1 {
		log.Println("wingo-contrib is a simple tool for managing contrib " +
			"scripts for the Wingo window manager.\n")
		log.Println("Usage:\n")
		log.Println("    wingo-contrib command [arguments]\n")
		log.Println("The commands are:\n")

		tabw := tabwriter.NewWriter(os.Stderr, 0, 4, 4, ' ', 0)
		for _, c := range cmds() {
			fmt.Fprintf(tabw, "    %s\t%s\n", c, commands[c].desc)
		}
		tabw.Flush()

		log.Println("\nTo uninstall a script, simply delete it.")
		os.Exit(1)
	}

	command = os.Args[1]
	if _, ok := commands[command]; !ok {
		log.Fatalf("Unknown subcommand %s does not exist.", command)
	}
	os.Args = os.Args[1:]
	commands[command].do()
}

func cmds() []string {
	cs := fun.Keys(commands).([]string)
	sort.Sort(sort.StringSlice(cs))
	return cs
}
