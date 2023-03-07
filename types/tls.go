package types

type TLSConfig struct {
	CaFile     string `mapstructure:"ca-file,omitempty"`
	KeyFile    string `mapstructure:"key-file,omitempty"`
	CertFile   string `mapstructure:"cert-file,omitempty"`
	SkipVerify bool   `mapstructure:"skip-verify,omitempty"`
}
