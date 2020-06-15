package file

import (
	"log"
	"os"
)

// Output //
type Output struct {
	cfg      *Config
	ch       chan []byte
	file     *os.File
	stopChan chan struct{}
}

// Config //
type Config struct {
	FileName   string
	BufferSize int
}

// NewOutput //
func NewOutput(c *Config) (*Output, error) {
	f, err := os.OpenFile(c.FileName, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}
	return &Output{
		cfg:  c,
		ch:   make(chan []byte, c.BufferSize),
		file: f,
	}, nil
}
func (f *Output) Write() error {
	for {
		select {
		case b := <-f.ch:
			_, err := f.file.Write(b)
			if err != nil {
				log.Printf("failed to write to file '%s': %v", f.file.Name(), err)
				continue
			}
		case <-f.stopChan:
			return nil
		}
	}
}

// Close //
func (f *Output) Close() error {
	close(f.ch)
	close(f.stopChan)
	return nil
}
