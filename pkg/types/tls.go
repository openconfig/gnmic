package types

import "fmt"

type TLSConfig struct {
	CaFile     string `mapstructure:"ca-file,omitempty"`
	KeyFile    string `mapstructure:"key-file,omitempty"`
	CertFile   string `mapstructure:"cert-file,omitempty"`
	SkipVerify bool   `mapstructure:"skip-verify,omitempty"`
	ClientAuth string `mapstructure:"client-auth,omitempty"`
}

func (t *TLSConfig) Validate() error {
	if t == nil {
		return nil
	}
	switch t.ClientAuth {
	case "", "request":
	case "require", "verify-if-given", "require-verify":
		if t.CaFile == "" {
			return fmt.Errorf("ca-file is required when `client-auth` is %q", t.ClientAuth)
		}
	default:
		return fmt.Errorf("unknown `client-auth` mode: %s", t.ClientAuth)
	}
	return nil
}
