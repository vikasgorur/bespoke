/*

Command hello demonstrates how to use the bespoke package. When run by itself
it prints an error message.

If it's turned into a bespoke binary with a map of the form
	{"name": "vikas"}

it prints "hello vikas".
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
