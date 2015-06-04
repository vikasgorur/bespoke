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
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kardianos/osext"
	"io"
	"io/ioutil"
	"path"
	"time"
)

const (
	mapFilename          = ".bespoke.json"
	fileHeaderLength     = 30    // + filename + extra (See https://golang.org/src/archive/zip/struct.go)
	filenameLengthOffset = 26    // offset of filename length within the file header
	extraLengthOffset    = 28    // offset of extra field length within the file header
	exeFileName          = "exe" // filename for the executable's entry in the zip file
)

type Bespoke struct {
	buffer    *bytes.Buffer // buffer that contains the zip archive
	archive   *zip.Writer   // zip archive
	finalized bool          // whether Close() has been called on archive
}

func newBespoke(exe io.Reader) (*Bespoke, error) {
	buffer := new(bytes.Buffer)
	archive := zip.NewWriter(buffer)

	w, err := archive.CreateHeader(&zip.FileHeader{
		Name:   exeFileName,
		Method: 0, // "Store", i.e., no compression
	})

	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(w, exe); err != nil {
		return nil, err
	}

	return &Bespoke{
		buffer:    buffer,
		archive:   archive,
		finalized: false,
	}, nil
}

func (b *Bespoke) addBuffer(p []byte, filename string) error {
	fh := &zip.FileHeader{Name: filename}
	fh.SetModTime(time.Now())
	fh.SetMode(0644)

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

	if err := discardFileHeader(b.buffer); err != nil {
		return err
	}
	b.finalized = true

	return nil
}

func (b *Bespoke) Read(p []byte) (int, error) {
	if !b.finalized {
		panic("read attempted without calling finalize")
	}

	return b.buffer.Read(p)
}

// Read and discard the file header for the executable
// so that the first thing read when the user calls Read() is the
// content of the executable itself.
func discardFileHeader(buf *bytes.Buffer) error {
	var header = make([]byte, fileHeaderLength)
	n, err := buf.Read(header)
	if err != nil || n < len(header) {
		return err
	}

	filenameLength := binary.LittleEndian.Uint16(header[filenameLengthOffset : filenameLengthOffset+2])
	extraFieldLength := binary.LittleEndian.Uint16(header[extraLengthOffset : extraLengthOffset+2])

	var moreHeader = make([]byte, filenameLength+extraFieldLength)
	n, err = buf.Read(moreHeader)
	if err != nil || n < len(moreHeader) {
		return err
	}

	return nil
}

func WithMap(exe io.Reader, data map[string]string) (*Bespoke, error) {
	content, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	b, err := newBespoke(exe)
	if err != nil {
		return nil, err
	}

	if err := b.addBuffer(content, mapFilename); err != nil {
		return nil, err
	}

	if err := b.finalize(); err != nil {
		return nil, err
	}

	return b, nil
}

func WithFile(exe io.Reader, filePath string) (*Bespoke, error) {
	b, err := newBespoke(exe)
	if err != nil {
		return nil, err
	}

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

func readFile(z *zip.ReadCloser, name string) ([]byte, error) {
	for _, f := range z.File {
		if f.Name == name {
			file, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer file.Close()

			content, err := ioutil.ReadAll(file)
			if err != nil {
				return nil, err
			}

			return content, nil
		}
	}

	return nil, errors.New("file not found in archive")
}

func Map() (map[string]string, error) {
	selfPath, err := osext.Executable()
	if err != nil {
		return nil, err
	}

	self, err := zip.OpenReader(selfPath)
	if err != nil {
		fmt.Println(err.Error(), selfPath)
		return nil, err
	}
	defer self.Close()

	content, err := readFile(self, mapFilename)
	if err != nil {
		return nil, err
	}

	var m map[string]string

	if err := json.Unmarshal(content, &m); err != nil {
		return nil, err
	}

	return m, nil
}
