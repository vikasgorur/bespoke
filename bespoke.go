/*

Package bespoke provides a way to create custom binaries: files that are executable
that also contain additional data. The data can either be a key-value map or an
arbitrary file.

Motivation

A common use for the Go language is creating command-line tools. As a concrete example,
consider a web application that allows its users to download a command-line
client to interact with it. The client may need to be configured with such
things as the user name or an access token. Using bespoke you can create a binary
that is specifically configured for each user who downloads it.
*/
package bespoke

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"github.com/kardianos/osext"
	"io"
	"io/ioutil"
	"math"
	"path"
	"time"
)

const (
	mapFilename = ".bespoke.json"

	// Offset values in the archive are uint32, and are of the form size(executable)+n
	// Restricting the executable size to 2^31 should be good enough.
	maxExecutableSize = math.MaxUint32 / 2

	directoryEndSignature = 0x06054b50

	// Offsets inside the EOCD table
	nDirRecordsOffset = 10 // offset of total number of central directory records)
	startOffset       = 16 // offset of start of central directory

	// Offsets inside a central directory file header
	filenameLengthOffset   = 28
	extraFieldLengthOffset = 30
	commentLengthOffset    = 32
	fhOffset               = 42
	fhFixedSize            = 46 // Size of the non-variable parts
)

// Bespoke represents a packaged bespoke binary. It is created by the functions
// bespoke.WithMap() and bespoke.WithFile(). It acts as an io.Reader and the contents
// of the bespoke binary can be accessed through Read().
type Bespoke struct {
	buffer    *bytes.Buffer // buffer that contains the zip archive
	archive   *zip.Writer   // zip archive
	exeLength uint32        // length of the executable that's stored at the beginning of buffer
	finalized bool          // whether Close() has been called on archive
}

func newBespoke(exe io.Reader) (*Bespoke, error) {
	buffer := new(bytes.Buffer)
	n, err := io.Copy(buffer, exe)
	if err != nil {
		return nil, err
	}

	if n > maxExecutableSize {
		return nil, errors.New("executables larger than 2^31 bytes not supported")
	}
	archive := zip.NewWriter(buffer)

	return &Bespoke{
		buffer:    buffer,
		archive:   archive,
		exeLength: uint32(n),
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

var le = binary.LittleEndian

// Return the offset of the end of central directory table.
func findEocdOffset(b []byte) int64 {
	sigBytes := make([]byte, 4)
	le.PutUint32(sigBytes, directoryEndSignature)

	for i := len(b) - 4; i > 0; i-- {
		if b[i] == sigBytes[0] &&
			b[i+1] == sigBytes[1] &&
			b[i+2] == sigBytes[2] &&
			b[i+3] == sigBytes[3] {
			return int64(i)
		}
	}

	return -1
}

// Return the size of the central directory file header record at b[off]
func fhRecordSize(b []byte, off uint32) uint32 {
	filenameLength := le.Uint16(b[off+filenameLengthOffset : off+filenameLengthOffset+2])
	extraLength := le.Uint16(b[off+extraFieldLengthOffset : off+extraFieldLengthOffset+2])
	commentLength := le.Uint16(b[off+commentLengthOffset : off+commentLengthOffset+2])

	return uint32(fhFixedSize + filenameLength + extraLength + commentLength)
}

// The offsets of files within the archive are wrong because we've prepended
// the executable. So add exeLength to every offset.
//
// This is equivalent to the --adjust-sfx option to the zip utility.
func (b *Bespoke) fixOffsets() error {
	buf := b.buffer.Bytes()
	eocd := findEocdOffset(buf)
	if eocd == -1 {
		return errors.New("couldn't find EOCD in archive")
	}

	nOff := eocd + nDirRecordsOffset
	n := int(le.Uint16(buf[nOff : nOff+2]))

	start := le.Uint32(buf[eocd+startOffset : eocd+startOffset+4])
	start += b.exeLength
	le.PutUint32(buf[eocd+startOffset:eocd+startOffset+4], start)

	h := start
	for i := 0; i < n; i++ {
		offset := le.Uint32(buf[h+fhOffset : h+fhOffset+4])
		offset += b.exeLength
		le.PutUint32(buf[h+fhOffset:h+fhOffset+4], offset)

		h += fhRecordSize(buf, h)
	}
	return nil
}

// Read reads the next len(p) bytes from the buffer or until the bespoke binary is drained.
// The return value n is the number of bytes read.
// If the binary has no data to return, err is io.EOF (unless len(p) is zero); otherwise it is nil.
func (b *Bespoke) Read(p []byte) (int, error) {
	if !b.finalized {
		panic("read attempted without calling finalize")
	}

	return b.buffer.Read(p)
}

// WithMap creates a bespoke binary from the executable exe and the given
// map. The executable can access the map by calling bespoke.Map()
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

// WithFile creates a bespoke binary from the executable exe and the given
// file. The executable can access the file by calling bespoke.ReadFile()
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

func openSelf() (*zip.ReadCloser, error) {
	selfPath, err := osext.Executable()
	if err != nil {
		return nil, err
	}

	self, err := zip.OpenReader(selfPath)
	if err != nil {
		return nil, errors.New(err.Error() + ": " + selfPath)
	}

	return self, nil
}

// ReadFile reads the given file that was packaged with the currently
// executing binary. It throws an error if this is not a bespoke binary
// or if the named file does not exist.
func ReadFile(name string) ([]byte, error) {
	self, err := openSelf()
	if err != nil {
		return nil, err
	}
	defer self.Close()

	return readFile(self, mapFilename)
}

// Map returns the string->string map that was packaged with the currently
// executing binary. It throws an error if this is not a bespoke binary.
func Map() (map[string]string, error) {
	self, err := openSelf()
	if err != nil {
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
