package file

import (
	"gopkg.in/natefinch/lumberjack.v2"
)

// RotationConfig manages configuration around file rotation
type rotationConfig struct {
	MaxSize    int  `mapstructure:"max-size,omitempty"`
	MaxBackups int  `mapstructure:"max-backups,omitempty"`
	MaxAge     int  `mapstructure:"max-age,omitempty"`
	Compress   bool `mapstructure:"compress,omitempty"`
}

func (r *rotationConfig) SetDefaults() {
	if r.MaxSize == 0 {
		r.MaxSize = 100
	}
	if r.MaxBackups == 0 {
		r.MaxBackups = 3
	}

	if r.MaxAge == 0 {
		r.MaxAge = 30
	}
}

type rotatingFile struct {
	l *lumberjack.Logger
}

// newRotatingFile initialize the lumberjack instance
func newRotatingFile(cfg *Config) *rotatingFile {
	cfg.Rotation.SetDefaults()

	lj := lumberjack.Logger{
		Filename:   cfg.FileName,
		MaxSize:    cfg.Rotation.MaxSize,
		MaxBackups: cfg.Rotation.MaxBackups,
		MaxAge:     cfg.Rotation.MaxAge,
		Compress:   cfg.Rotation.Compress,
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
