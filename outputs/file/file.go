package file

import (
	"log"
	"os"

	"github.com/karimra/gnmiClient/outputs"
	"github.com/mitchellh/mapstructure"
)

func init() {
	outputs.Register("file", func() outputs.Output {
		return &File{
			Cfg: &Config{},
		}
	})
}

// File //
type File struct {
	Cfg      *Config
	file     *os.File
	stopChan chan struct{}
}

// Config //
type Config struct {
	FileName string
}

// Initialize //
func (f *File) Initialize(cfg map[string]interface{}) error {
	c := new(Config)
	err := mapstructure.Decode(cfg, c)
	if err != nil {
		return err
	}
	f.Cfg = c
	file, err := os.OpenFile(f.Cfg.FileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	f.file = file
	f.stopChan = make(chan struct{})
	return nil
}

// Write //
func (f *File) Write(b []byte) {
	_, err := f.file.Write(append(b, []byte("\n")...))
	if err != nil {
		log.Printf("failed to write to file '%s': %v", f.file.Name(), err)
		return
	}
}

// Close //
func (f *File) Close() error {
	close(f.stopChan)
	return nil
}
