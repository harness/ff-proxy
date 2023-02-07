package ffproxy

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type fileExtension string

const (
	extensionJSON = ".json"
	extensionYAML = ".yaml"
)

// DecodeFile is a convienence function that creates a FileDecoder and calls Decode
func DecodeFile(fileSystem fs.FS, path string, v interface{}) error {
	dec, err := NewFileDecoder(fileSystem, path)
	if err != nil {
		return err
	}

	return dec.Decode(v)
}

type decoder interface {
	Decode(v interface{}) error
}

// FileDecoder is a type that can be used to decodes files of different formats
// into a go type.
type FileDecoder struct {
	file io.Closer
	dec  decoder
}

// NewFileDecoder opens the file and creates a decoder based on the file extension.
// If the file extension is not supported it returns an error. NewFileDecoder does
// not close the opened file, for the file to be closed you have to call the Decode
// method.
func NewFileDecoder(fileSystem fs.FS, file string) (*FileDecoder, error) {
	f, err := fileSystem.Open(file)
	if err != nil {
		return nil, err
	}

	ext := filepath.Ext(file)
	var dec decoder

	switch ext {
	case extensionJSON:
		dec = json.NewDecoder(f)
	case extensionYAML:
		dec = yaml.NewDecoder(f)
	default:
		return nil, fmt.Errorf("unsupported file extension %s, supported extensions are %s, %s", ext, extensionJSON, extensionYAML)
	}

	return &FileDecoder{file: f, dec: dec}, nil
}

// Decode decodes the contents of the file into v and closes it
func (f *FileDecoder) Decode(v interface{}) error {
	//#nosec G307
	defer f.file.Close()
	return f.dec.Decode(v)
}
