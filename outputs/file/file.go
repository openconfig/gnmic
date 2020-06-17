package file

import (
	"fmt"
	"log"
	"os"

	"github.com/karimra/gnmiClient/outputs"
	"github.com/mitchellh/mapstructure"
)

func init() {
	fmt.Println("init file output")
	outputs.Register("file", func() outputs.Output {
		return &File{
			Cfg: &Config{},
		}
	})
}

// File //
type File struct {
	Cfg      *Config
	ch       chan []byte
	file     *os.File
	stopChan chan struct{}
}

// Config //
type Config struct {
	FileName   string
	BufferSize int
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
	f.ch = make(chan []byte, f.Cfg.BufferSize)
	return nil
}

// Start //
func (f *File) Start() {
	for {
		select {
		case b := <-f.ch:
			_, err := f.file.Write(b)
			if err != nil {
				log.Printf("failed to write to file '%s': %v", f.file.Name(), err)
				continue
			}
			_, err = f.file.WriteString("\n")
		case <-f.stopChan:
			return
		}
	}
}

// Write //
func (f *File) Write(b []byte) {
	select {
	default:
		f.ch <- b
	case <-f.stopChan:
		return
	}
}

// Close //
func (f *File) Close() error {
	close(f.ch)
	close(f.stopChan)
	return nil
}
