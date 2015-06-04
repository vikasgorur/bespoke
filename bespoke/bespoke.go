package main

import (
	"flag"
	"github.com/vikasgorur/bespoke"
	"io"
	"os"
)

func main() {
	var name = flag.String("name", "vikas", "name to add to the executable")
	flag.Parse()
	args := flag.Args()

	if len(args) != 2 {
		flag.Usage()
		os.Exit(1)
	}

	exe, err := os.Open(args[0])
	if err != nil {
		panic(err.Error())
	}

	b, err := bespoke.WithMap(exe, map[string]string{"name": *name})
	if err != nil {
		panic(err.Error())
	}

	out, err := os.OpenFile(args[1], os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		panic(err.Error())
	}
	defer out.Close()

	if _, err := io.Copy(out, b); err != nil {
		panic(err.Error())
	}
}
