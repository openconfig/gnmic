package types

type SASL struct {
	User      string `mapstructure:"user,omitempty"`
	Password  string `mapstructure:"password,omitempty"`
	Mechanism string `mapstructure:"mechanism,omitempty"`
	TokenURL  string `mapstructure:"token-url,omitempty"`
}
