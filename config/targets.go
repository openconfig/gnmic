package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"

	"github.com/karimra/gnmic/collector"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/crypto/ssh/terminal"
)

func (c *Config) GetTargets() (map[string]*collector.TargetConfig, error) {
	targets := make(map[string]*collector.TargetConfig)
	defGrpcPort := c.FileConfig.GetString("port")
	// case address is defined in .Address
	if len(c.Globals.Address) > 0 {
		if c.Globals.Username == "" {
			defUsername, err := readUsername()
			if err != nil {
				return nil, err
			}
			c.Globals.Username = defUsername
		}
		if c.Globals.Password == "" {
			defPassword, err := readPassword()
			if err != nil {
				return nil, err
			}
			c.Globals.Password = defPassword
		}
		for _, addr := range c.Globals.Address {
			tc := new(collector.TargetConfig)
			if !strings.HasPrefix(addr, "unix://") {
				_, _, err := net.SplitHostPort(addr)
				if err != nil {
					if strings.Contains(err.Error(), "missing port in address") ||
						strings.Contains(err.Error(), "too many colons in address") {
						addr = net.JoinHostPort(addr, defGrpcPort)
					} else {
						c.logger.Printf("error parsing address '%s': %v", addr, err)
						return nil, fmt.Errorf("error parsing address '%s': %v", addr, err)
					}
				}
			}
			tc.Address = addr
			c.setTargetConfigDefaults(tc)
			targets[tc.Name] = tc
		}
		if c.Globals.Debug {
			c.logger.Printf("targets: %v", targets)
		}
		return targets, nil
	}
	// case targets is defined in config file
	targetsInt := c.FileConfig.Get("targets")
	targetsMap := make(map[string]interface{})
	switch targetsInt := targetsInt.(type) {
	case string:
		for _, addr := range strings.Split(targetsInt, " ") {
			targetsMap[addr] = nil
		}
	case map[string]interface{}:
		targetsMap = targetsInt
	case nil:
		return nil, errors.New("no targets found")
	default:
		return nil, fmt.Errorf("unexpected targets format, got: %T", targetsInt)
	}
	if len(targetsMap) == 0 {
		return nil, fmt.Errorf("no targets found")
	}
	for addr, t := range targetsMap {
		if !strings.HasPrefix(addr, "unix://") {
			_, _, err := net.SplitHostPort(addr)
			if err != nil {
				if strings.Contains(err.Error(), "missing port in address") ||
					strings.Contains(err.Error(), "too many colons in address") {
					addr = net.JoinHostPort(addr, defGrpcPort)
				} else {
					c.logger.Printf("error parsing address '%s': %v", addr, err)
					return nil, fmt.Errorf("error parsing address '%s': %v", addr, err)
				}
			}
		}
		tc := new(collector.TargetConfig)
		switch t := t.(type) {
		case map[string]interface{}:
			decoder, err := mapstructure.NewDecoder(
				&mapstructure.DecoderConfig{
					DecodeHook: mapstructure.StringToTimeDurationHookFunc(),
					Result:     tc,
				},
			)
			if err != nil {
				return nil, err
			}
			err = decoder.Decode(t)
			if err != nil {
				return nil, err
			}
		case nil:
		default:
			return nil, fmt.Errorf("unexpected targets format, got a %T", t)
		}
		tc.Address = addr
		c.setTargetConfigDefaults(tc)
		if c.Globals.Debug {
			c.logger.Printf("read target config: %s", tc)
		}
		targets[tc.Name] = tc
	}
	subNames := c.FileConfig.GetStringSlice("subscribe-name")
	if len(subNames) == 0 {
		if c.Globals.Debug {
			c.logger.Printf("targets: %v", targets)
		}
		return targets, nil
	}
	for n := range targets {
		targets[n].Subscriptions = subNames
	}
	if c.Globals.Debug {
		c.logger.Printf("targets: %v", targets)
	}
	return targets, nil
}

func readUsername() (string, error) {
	var username string
	fmt.Print("username: ")
	_, err := fmt.Scan(&username)
	if err != nil {
		return "", err
	}
	return username, nil
}
func readPassword() (string, error) {
	fmt.Print("password: ")
	pass, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	fmt.Println()
	return string(pass), nil
}

func (c *Config) setTargetConfigDefaults(tc *collector.TargetConfig) {
	if tc.Name == "" {
		tc.Name = tc.Address
	}
	if tc.Username == nil {
		tc.Username = &c.Globals.Username
	}
	if tc.Password == nil {
		tc.Password = &c.Globals.Password
	}
	if tc.Timeout == 0 {
		tc.Timeout = c.Globals.Timeout
	}
	if tc.Insecure == nil {
		tc.Insecure = &c.Globals.Insecure
	}
	if tc.SkipVerify == nil {
		tc.SkipVerify = &c.Globals.SkipVerify
	}
	if tc.Insecure != nil && !*tc.Insecure {
		if tc.TLSCA == nil {
			if c.Globals.TLSCa != "" {
				tc.TLSCA = &c.Globals.TLSCa
			}
		}
		if tc.TLSCert == nil {
			tc.TLSCert = &c.Globals.TLSCert
		}
		if tc.TLSKey == nil {
			tc.TLSKey = &c.Globals.TLSKey
		}
	}
	if tc.RetryTimer == 0 {
		tc.RetryTimer = c.Globals.Retry
	}
	if tc.TLSVersion == "" {
		tc.TLSVersion = c.Globals.TLSVersion
	}
	if tc.TLSMinVersion == "" {
		tc.TLSMinVersion = c.Globals.TLSMinVersion
	}
	if tc.TLSMaxVersion == "" {
		tc.TLSMaxVersion = c.Globals.TLSMaxVersion
	}
}

func (c *Config) TargetsList() []*collector.TargetConfig {
	targetsMap, err := c.GetTargets()
	if err != nil {
		return nil
	}
	targets := make([]*collector.TargetConfig, 0, len(targetsMap))
	for _, tc := range targetsMap {
		targets = append(targets, tc)
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Name < targets[j].Name
	})
	return targets
}
