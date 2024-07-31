package file

import (
	"fmt"

	"gopkg.in/natefinch/lumberjack.v2"
)

// RotationConfig manages configuration around file rotation
type RotationConfig struct {
	MaxSize    int  `mapstructure:"max-size,omitempty"`
	MaxBackups int  `mapstructure:"max-backups,omitempty"`
	MaxAge     int  `mapstructure:"max-age,omitempty"`
	Compress   bool `mapstructure:"compress,omitempty"`
}

// validateConfig ensures all parameters are supplied
func (rc *RotationConfig) validateConfig() error {
	if rc.MaxSize == 0 {
		return fmt.Errorf("Rotation.MaxSize is required if using type 'rotating'")
	}

	if rc.MaxBackups == 0 {
		return fmt.Errorf("Rotation.MaxBackups is required if using type 'rotating'")
	}

	if rc.MaxAge == 0 {
		return fmt.Errorf("Rotation.MaxAge is required if using type 'rotating'")
	}

	return nil
}

type rotatingFile struct {
	l *lumberjack.Logger
}

// newRotatingFile initialize the lumberjack instance
func newRotatingFile(filename string, compress bool, maxSize, maxBackups, maxAge int) *rotatingFile {
	lj := lumberjack.Logger{
		Filename:   filename,
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
		Compress:   compress,
	}

	return &rotatingFile{l: &lj}
}

// Close closes the file
func (r *rotatingFile) Close() error {
	return r.l.Close()
}

// Name returns the name of the file
func (r *rotatingFile) Name() string {
	return r.l.Filename
}

// Write implements io.Writer
func (r *rotatingFile) Write(b []byte) (int, error) {
	return r.l.Write(b)
}
