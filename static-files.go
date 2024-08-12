package steamscreenshots

import (
	"os"
	//"fmt"
	"embed"
	"io/fs"
	//"io"
	"bytes"
	"time"
)

//go:embed static templates
var embeddedContent embed.FS

type staticFiles struct {
	files map[string]fs.File
}

func (sf *staticFiles) Open(name string) (fs.File, error) {
	file, found := sf.files[name]
	if found {
		if f, ok := file.(*staticFile); ok {
			return f.Open(), nil
		}
		return file, nil
	}

	return embeddedContent.Open(name)

	//return nil, fs.ErrNotExist
}

func (sf *staticFiles) LoadFromDisk(source, destination string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}

	raw, err := os.ReadFile(source)
	if err != nil {
		return err
	}

	sf.files[destination] = newStaticFile(info, raw)
	return nil
}

func (sf *staticFiles) LoadFromFS(source, destination string, filesystem fs.FS) {
}

type staticFile struct {
	data []byte
	info fs.FileInfo
	reader *bytes.Reader
}

func newStaticFile(info fs.FileInfo, data []byte) fs.File {
	return &staticFile{
		data: data,
		info: info,
		reader: bytes.NewReader(data),
	}
}

func (sf *staticFile) Open() fs.File {
	sf.reader.Reset(sf.data)
	return sf
}

func (sf *staticFile) Stat() (fs.FileInfo, error) {
	return sf.info, nil
}

func (sf *staticFile) Read(buf []byte) (int, error) {
	return sf.reader.Read(buf)
}

func (sf *staticFile) Close() error {
	return nil
}

type staticInfo struct {
	name string
	size int64
	mode fs.FileMode
	modTime time.Time
}

func (si *staticInfo) Name()  string      { return si.name }
func (si *staticInfo) Size()  int64       { return si.size }
func (si *staticInfo) Mode()  fs.FileMode { return si.mode }
func (si *staticInfo) IsDir() bool        { return false }
func (si *staticInfo) Sys()   any         { return nil }
