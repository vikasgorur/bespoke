/*

Package bespoke provides a way to create custom binaries: files that are executable
that also contain additional data. The data can be anything --- a simple key-value map,
a text file, or an entire directory tree.

Motivation

A common use for the Go language is creating command-line tools. As a concrete example,
consider a web application that allows its users to download a command-line
client to interact with it. The client may need to be configured with such
things as the user name, or perhaps an access token. Instead of making the user
edit a text file or supply these as options, why not deliver a binary
that was customized ("bespoke") for her and already included all the
user-specific data?

A secondary use for this package is to create "bundles" for deploying web apps that
contain both the application binary and static assets (html, css, js, images). This is
similar to how a Java application would be packaged into a JAR.
*/
package bespoke

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"path"
)

type Bespoke struct {
	executable io.Reader     // the executable to be customized
	buffer     *bytes.Buffer // buffer that contains the zip archive
	archive    *zip.Writer   // zip archive
	bundle     io.Reader     // the finalized executable
	finalized  bool          // whether Close() has been called on archive
}

const MAP_FILENAME = ".bespoke.json"

func newBespoke(exe io.Reader) *Bespoke {
	buffer := new(bytes.Buffer)
	return &Bespoke{
		executable: exe,
		buffer:     buffer,
		archive:    zip.NewWriter(buffer),
		finalized:  false,
	}
}

func (b *Bespoke) addBuffer(p []byte, filename string) error {
	fh := &zip.FileHeader{Name: filename}

	w, err := b.archive.CreateHeader(fh)
	if err != nil {
		return err
	}

	n, err := w.Write(p)
	if err != nil || n < len(p) {
		return errors.New("could not add to archive " + err.Error())
	}

	return nil
}

func (b *Bespoke) addFile(p string) error {
	filename := path.Base(p)

	content, err := ioutil.ReadFile(p)
	if err != nil {
		return err
	}

	return b.addBuffer(content, filename)
}

func (b *Bespoke) finalize() error {
	if err := b.archive.Close(); err != nil {
		return err
	}

	b.bundle = io.MultiReader(b.executable, b.buffer)
	b.finalized = true

	return nil
}

func (b *Bespoke) Read(p []byte) (int, error) {
	if !b.finalized {
		panic("read attempted without calling finalize")
	}

	return b.bundle.Read(p)
}

func WithMap(exe io.Reader, data map[string]string) (*Bespoke, error) {
	content, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	b := newBespoke(exe)
	if err := b.addBuffer(content, MAP_FILENAME); err != nil {
		return nil, err
	}

	if err := b.finalize(); err != nil {
		return nil, err
	}

	return b, nil
}

func WithFile(exe io.Reader, filePath string) (*Bespoke, error) {
	b := newBespoke(exe)
	if err := b.addFile(filePath); err != nil {
		return nil, err
	}

	if err := b.finalize(); err != nil {
		return nil, err
	}

	return b, nil
}

func WithDir(exe io.Reader, dirPath string) (*Bespoke, error) {
	return nil, nil
}
