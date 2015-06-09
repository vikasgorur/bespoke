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
	mapFilename = ".bespoke.json"

	directoryEndSignature = 0x06054b50
)

type Bespoke struct {
	buffer    *bytes.Buffer // buffer that contains the zip archive
	archive   *zip.Writer   // zip archive
	exeLength int64         // length of the executable that's stored at the beginning of buffer
	finalized bool          // whether Close() has been called on archive
}

func newBespoke(exe io.Reader) (*Bespoke, error) {
	buffer := new(bytes.Buffer)
	n, err := io.Copy(buffer, exe)
	if err != nil {
		return nil, err
	}

	archive := zip.NewWriter(buffer)

	return &Bespoke{
		buffer:    buffer,
		archive:   archive,
		exeLength: n,
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

// Write the archive to the buffer and fix up offsets.
func (b *Bespoke) finalize() error {
	if err := b.archive.Close(); err != nil {
		return err
	}

	if err := b.fixOffsets(); err != nil {
		return err
	}
	b.finalized = true

	return nil
}

func findEocdOffset(b []byte) int64 {
	sigBytes := make([]byte, 4)
	binary.LittleEndian.PutUint64(sigBytes, directoryEndSignature)

	for i := len(b) - 4; i > 0; i-- {
		if b[i] == sigBytes[0] &&
			b[i+1] == sigBytes[1] &&
			b[i+2] == sigBytes[i+2] &&
			b[i+3] == sigBytes[i+4] {
			return int64(i)
		}
	}

	return -1
}

// The offsets of files within the archive are wrong because we've prepended
// the executable. So add exeLength to every offset.
//
// This is equivalent to the --adjust-sfx option to the zip utility.
func (b *Bespoke) fixOffsets() error {
	fmt.Println(findEocdOffset(b.buffer.Bytes()))
	return nil
}

func (b *Bespoke) Read(p []byte) (int, error) {
	if !b.finalized {
		panic("read attempted without calling finalize")
	}

	return b.buffer.Read(p)
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
