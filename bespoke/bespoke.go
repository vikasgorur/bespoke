package main

import (
	"flag"
	"fmt"
	"github.com/vikasgorur/bespoke"
	"io"
	"os"
)

func usage() {
	fmt.Println("exe file output")
}

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) != 3 {
		usage()
		os.Exit(1)
	}

	exe, err := os.Open(args[0])
	if err != nil {
		panic(err.Error())
	}

	b, err := bespoke.WithFile(exe, args[1])
	if err != nil {
		panic(err.Error())
	}

	out, err := os.Create(args[2])
	if err != nil {
		panic(err.Error())
	}
	defer out.Close()

	if _, err := io.Copy(out, b); err != nil {
		panic(err.Error())
	}
}
