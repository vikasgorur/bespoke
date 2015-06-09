/*
Command server is a simple HTTP server that demonstrates usage of the bespoke package.

Usage

Start the server from the bespoke root directory, after making sure that the
hello command has been built.

  $ examples/server/server
  2015/06/09 00:11:12 listening at http://localhost:6060/

Now download and execute a bespoke binary by doing

  $ curl http://localhost:6060/foo > hello_foo
  $ chmod +x hello_foo
  $ ./hello_foo
  hello foo
*/
package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/vikasgorur/bespoke"
	"io"
	"log"
	"net/http"
	"os"
)

func handleName(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	m := map[string]string{"name": name}

	exe, err := os.Open("examples/hello/hello")
	if err != nil {
		fmt.Fprintf(w, err.Error())
		w.WriteHeader(404)
		return
	}
	defer exe.Close()

	b, err := bespoke.WithMap(exe, m)
	if err != nil {
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	io.Copy(w, b)
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/{name}", handleName).Methods("GET")

	http.Handle("/", r)

	log.Println("listening at http://localhost:6060/")
	http.ListenAndServe(":6060", nil)
}
