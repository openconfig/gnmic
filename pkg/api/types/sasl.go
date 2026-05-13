package types

import (
	"log/slog"
)

type SASL struct {
	User      string `mapstructure:"user,omitempty"`
	Password  string `mapstructure:"password,omitempty"`
	Mechanism string `mapstructure:"mechanism,omitempty"`
	TokenURL  string `mapstructure:"token-url,omitempty"`
}

func (s *SASL) LogValue() slog.Value {
	if s == nil {
		return slog.StringValue("<nil>")
	}
	return slog.GroupValue(
		slog.String("user", s.User),
		slog.String("mechanism", s.Mechanism),
		slog.String("token-url", s.TokenURL),
	)
}
