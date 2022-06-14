package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"

	"github.com/karimra/gnmic/types"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/term"
)

const (
	defaultTargetBufferSize = 100
)

var ErrNoTargetsFound = errors.New("no targets found")

func (c *Config) GetTargets() (map[string]*types.TargetConfig, error) {
	var err error
	// case address is defined in .Address
	if len(c.Address) > 0 {
		if c.Username == "" && c.Token == "" {
			defUsername, err := readUsername()
			if err != nil {
				return nil, err
			}
			c.Username = defUsername
		}
		if c.Password == "" && c.Token == "" {
			defPassword, err := readPassword()
			if err != nil {
				return nil, err
			}
			c.Password = defPassword
		}

		for _, addr := range c.Address {
			tc := &types.TargetConfig{
				Name:    addr,
				Address: addr,
			}
			err = c.SetTargetConfigDefaults(tc)
			if err != nil {
				return nil, err
			}
			c.Targets[tc.Name] = tc
		}
		if c.Debug {
			c.logger.Printf("targets: %v", c.Targets)
		}
		return c.Targets, nil
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
		return nil, ErrNoTargetsFound
	default:
		return nil, fmt.Errorf("unexpected targets format, got: %T", targetsInt)
	}
	if len(targetsMap) == 0 {
		return nil, ErrNoTargetsFound
	}

	newTargetsConfig := make(map[string]*types.TargetConfig)
	for name, t := range targetsMap {
		tc := new(types.TargetConfig)
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
		if tc.Address == "" {
			tc.Address = name
		}
		if tc.Name == "" {
			tc.Name = name
		}
		err = c.SetTargetConfigDefaults(tc)
		if err != nil {
			return nil, err
		}
		if c.Debug {
			c.logger.Printf("read target config: %s", tc)
		}
		err = expandCertPaths(tc)
		if err != nil {
			return nil, err
		}
		// due to a viper bug that changes env values to lowercase if read
		// as part of a StringMap or interface{}:
		// read the target password as a string to maintain its case.
		// if it's not an empty string set it explicitly
		pass := c.FileConfig.GetString(fmt.Sprintf("targets/%s/password", name))
		if pass != "" {
			*tc.Password = pass
		}
		expandTargetEnv(tc)
		newTargetsConfig[name] = tc
	}
	c.Targets = newTargetsConfig

	subNames := c.FileConfig.GetStringSlice("subscribe-name")
	if len(subNames) == 0 {
		if c.Debug {
			c.logger.Printf("targets: %v", c.Targets)
		}
		return c.Targets, nil
	}
	for n := range c.Targets {
		c.Targets[n].Subscriptions = subNames
	}
	if c.Debug {
		c.logger.Printf("targets: %v", c.Targets)
	}
	return c.Targets, nil
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
	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	fmt.Println()
	return string(pass), nil
}

func (c *Config) SetTargetConfigDefaults(tc *types.TargetConfig) error {
	defGrpcPort := c.FileConfig.GetString("port")
	if !strings.HasPrefix(tc.Address, "unix://") {
		addrList := strings.Split(tc.Address, ",")
		addrs := make([]string, 0, len(addrList))
		for _, addr := range addrList {
			addr = strings.TrimSpace(addr)
			if !c.UseTunnelServer {
				_, _, err := net.SplitHostPort(addr)
				if err != nil {
					if strings.Contains(err.Error(), "missing port in address") ||
						strings.Contains(err.Error(), "too many colons in address") {
						addr = net.JoinHostPort(addr, defGrpcPort)
					} else {
						c.logger.Printf("error parsing address '%s': %v", addr, err)
						return fmt.Errorf("error parsing address '%s': %v", addr, err)
					}
				}
			}
			addrs = append(addrs, addr)
		}
		tc.Address = strings.Join(addrs, ",")
	}
	if tc.Username == nil {
		tc.Username = &c.Username
	}
	if tc.Password == nil {
		tc.Password = &c.Password
	}
	if tc.Token == nil {
		tc.Token = &c.Token
	}
	if tc.Timeout == 0 {
		tc.Timeout = c.Timeout
	}
	if tc.Insecure == nil {
		tc.Insecure = &c.Insecure
	}
	if tc.SkipVerify == nil {
		tc.SkipVerify = &c.SkipVerify
	}
	if tc.Insecure != nil && !*tc.Insecure {
		if tc.TLSCA == nil {
			if c.TLSCa != "" {
				tc.TLSCA = &c.TLSCa
			}
		}
		if tc.TLSCert == nil {
			tc.TLSCert = &c.TLSCert
		}
		if tc.TLSKey == nil {
			tc.TLSKey = &c.TLSKey
		}
	}
	if tc.RetryTimer == 0 {
		tc.RetryTimer = c.Retry
	}
	if tc.TLSVersion == "" {
		tc.TLSVersion = c.TLSVersion
	}
	if tc.TLSMinVersion == "" {
		tc.TLSMinVersion = c.TLSMinVersion
	}
	if tc.TLSMaxVersion == "" {
		tc.TLSMaxVersion = c.TLSMaxVersion
	}
	if tc.LogTLSSecret == nil {
		tc.LogTLSSecret = &c.LogTLSSecret
	}
	if tc.Gzip == nil {
		tc.Gzip = &c.Gzip
	}
	if tc.BufferSize == 0 {
		tc.BufferSize = defaultTargetBufferSize
	}
	return nil
}

func (c *Config) TargetsList() []*types.TargetConfig {
	targets := make([]*types.TargetConfig, 0, len(c.Targets))
	for _, tc := range c.Targets {
		targets = append(targets, tc)
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Name < targets[j].Name
	})
	return targets
}

func expandCertPaths(tc *types.TargetConfig) error {
	if tc.Insecure != nil && !*tc.Insecure {
		var err error
		if tc.TLSCA != nil && *tc.TLSCA != "" {
			*tc.TLSCA, err = expandOSPath(*tc.TLSCA)
			if err != nil {
				return err
			}

		}
		if tc.TLSCert != nil && *tc.TLSCert != "" {
			*tc.TLSCert, err = expandOSPath(*tc.TLSCert)
			if err != nil {
				return err
			}

		}
		if tc.TLSKey != nil && *tc.TLSKey != "" {
			*tc.TLSKey, err = expandOSPath(*tc.TLSKey)
			if err != nil {
				return err
			}

		}
	}
	return nil
}

func expandTargetEnv(tc *types.TargetConfig) {
	tc.Name = os.ExpandEnv(tc.Name)
	tc.Address = os.ExpandEnv(tc.Address)
	if tc.Username != nil {
		*tc.Username = os.ExpandEnv(*tc.Username)
	}
	// expandEnv for the pasword field only if it starts with $
	// https://github.com/karimra/gnmic/issues/496
	if tc.Password != nil && strings.HasPrefix(*tc.Password, "$") {
		*tc.Password = os.ExpandEnv(*tc.Password)
	}
	if tc.Token != nil {
		*tc.Token = os.ExpandEnv(*tc.Token)
	}
	if tc.TLSCA != nil {
		*tc.TLSCA = os.ExpandEnv(*tc.TLSCA)
	}
	if tc.TLSCert != nil {
		*tc.TLSCert = os.ExpandEnv(*tc.TLSCert)
	}
	if tc.TLSKey != nil {
		*tc.TLSKey = os.ExpandEnv(*tc.TLSKey)
	}
	for i := range tc.Subscriptions {
		tc.Subscriptions[i] = os.ExpandEnv(tc.Subscriptions[i])
	}
	for i := range tc.Outputs {
		tc.Outputs[i] = os.ExpandEnv(tc.Outputs[i])
	}
	tc.TLSMinVersion = os.ExpandEnv(tc.TLSMinVersion)
	tc.TLSMaxVersion = os.ExpandEnv(tc.TLSMaxVersion)
	tc.TLSVersion = os.ExpandEnv(tc.TLSVersion)
	for i := range tc.ProtoFiles {
		tc.ProtoFiles[i] = os.ExpandEnv(tc.ProtoFiles[i])
	}
	for i := range tc.ProtoDirs {
		tc.ProtoDirs[i] = os.ExpandEnv(tc.ProtoDirs[i])
	}
	for i := range tc.Tags {
		tc.Tags[i] = os.ExpandEnv(tc.Tags[i])
	}
}

func (c *Config) GetDiffTargets() (*types.TargetConfig, map[string]*types.TargetConfig, error) {
	targetsConfig, err := c.GetTargets()
	if err != nil {
		if err != ErrNoTargetsFound {
			return nil, nil, err
		}
	}
	var refConfig *types.TargetConfig
	if rc, ok := targetsConfig[c.DiffRef]; ok {
		refConfig = rc
	} else {
		refConfig = &types.TargetConfig{
			Name:    c.DiffRef,
			Address: c.DiffRef,
		}
		err = c.SetTargetConfigDefaults(refConfig)
		if err != nil {
			return nil, nil, err
		}
	}
	compareConfigs := make(map[string]*types.TargetConfig)
	for _, cmp := range c.DiffCompare {
		if cc, ok := targetsConfig[cmp]; ok {
			compareConfigs[cmp] = cc
		} else {
			compConfig := &types.TargetConfig{
				Name:    cmp,
				Address: cmp,
			}
			err = c.SetTargetConfigDefaults(compConfig)
			if err != nil {
				return nil, nil, err
			}
			compareConfigs[compConfig.Name] = compConfig
		}
	}
	return refConfig, compareConfigs, nil
}
