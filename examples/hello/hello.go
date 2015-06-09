/*
Command hello is a simple example of a bespoke binary. When run by itself
it prints an error message.

If it's turned into a bespoke binary with a map of the form
	{"name": "vikas"}

it prints "hello vikas".

See examples/server for a HTTP server that uses this binary.
*/
package main

import (
	"fmt"
	"github.com/vikasgorur/bespoke"
)

func main() {
	m, err := bespoke.Map()
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("hello " + m["name"])
	}
}
