package file_loader

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/karimra/gnmic/loaders"
	"github.com/karimra/gnmic/types"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"gopkg.in/yaml.v2"
)

const (
	loggingPrefix   = "[file_loader] "
	watchInterval   = 5 * time.Second
	loaderType      = "file"
	defaultFTPPort  = 21
	defaultSFTPPort = 22
)

func init() {
	loaders.Register(loaderType, func() loaders.TargetLoader {
		return &fileLoader{
			cfg:         &cfg{},
			lastTargets: make(map[string]*types.TargetConfig),
			logger:      log.New(ioutil.Discard, loggingPrefix, log.LstdFlags|log.Lmicroseconds),
		}
	})
}

type fileLoader struct {
	cfg         *cfg
	lastTargets map[string]*types.TargetConfig
	logger      *log.Logger
}

type cfg struct {
	Path     string        `json:"path,omitempty" mapstructure:"path,omitempty"`
	Interval time.Duration `json:"interval,omitempty" mapstructure:"interval,omitempty"`
}

func (f *fileLoader) Init(ctx context.Context, cfg map[string]interface{}, logger *log.Logger) error {
	err := loaders.DecodeConfig(cfg, f.cfg)
	if err != nil {
		return err
	}
	if f.cfg.Path == "" {
		return errors.New("missing file path")
	}
	if f.cfg.Interval <= 0 {
		f.cfg.Interval = watchInterval
	}
	if logger != nil {
		f.logger.SetOutput(logger.Writer())
		f.logger.SetFlags(logger.Flags())
	}
	return nil
}

func (f *fileLoader) Start(ctx context.Context) chan *loaders.TargetOperation {
	opChan := make(chan *loaders.TargetOperation)
	go func() {
		defer close(opChan)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				readTargets, err := f.getTargets()
				if _, ok := err.(*os.PathError); ok {
					time.Sleep(f.cfg.Interval)
					continue
				}
				if err != nil {
					f.logger.Printf("failed to read targets file: %v", err)
					time.Sleep(f.cfg.Interval)
					continue
				}
				select {
				case <-ctx.Done():
					return
				case opChan <- f.diff(readTargets):
					time.Sleep(f.cfg.Interval)
				}
			}
		}
	}()
	return opChan
}

func (f *fileLoader) getTargets() (map[string]*types.TargetConfig, error) {
	switch {
	case strings.HasPrefix(f.cfg.Path, "https://"):
		fallthrough
	case strings.HasPrefix(f.cfg.Path, "http://"):
		return f.readHTTPFile()
	case strings.HasPrefix(f.cfg.Path, "ftp://"):
		return f.readFTPFile()
	case strings.HasPrefix(f.cfg.Path, "sftp://"):
		return f.readSFTPFile()
	default:
		return f.readLocalFile()
	}
}

// readHTTPFile fetches a remote from from an HTTP server,
// the resoonse body can be yaml or json bytes.
// it then unmarshal the received bytes into a map[string]*types.TargetConfig
// and returns
func (f *fileLoader) readHTTPFile() (map[string]*types.TargetConfig, error) {
	_, err := url.Parse(f.cfg.Path)
	if err != nil {
		return nil, err
	}
	client := http.Client{
		Timeout: f.cfg.Interval / 2,
	}
	if strings.HasPrefix(f.cfg.Path, "https://") {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
	r, err := client.Get(f.cfg.Path)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	readBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	result := make(map[string]*types.TargetConfig)
	err = yaml.Unmarshal(readBody, result)
	return result, err
}

// readFTPFile reads a file from a remote FTP server
// unmarshals the content into a map[string]*types.TargetConfig
// and returns
func (f *fileLoader) readFTPFile() (map[string]*types.TargetConfig, error) {
	parsedUrl, err := url.Parse(f.cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %v", err)
	}

	// Get user name and pass
	user := parsedUrl.User.Username()
	pass, _ := parsedUrl.User.Password()

	// Parse Host and Port
	host := parsedUrl.Host

	// connect to server
	conn, err := ftp.Dial(fmt.Sprintf("%s:%d", host, 21), ftp.DialWithTimeout(f.cfg.Interval/2))
	if err != nil {
		return nil, fmt.Errorf("failed to connecto to [%s]: %v", host, err)
	}

	err = conn.Login(user, pass)
	if err != nil {
		return nil, fmt.Errorf("failed to login to [%s]: %v", host, err)
	}

	r, err := conn.Retr(parsedUrl.RequestURI())
	if err != nil {
		return nil, fmt.Errorf("failed to read remtoe file %q: %v", parsedUrl.RequestURI(), err)
	}
	defer r.Close()

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read remtoe file %q: %v", parsedUrl.RequestURI(), err)
	}
	// unmarshal the received bytes and return the result
	result := make(map[string]*types.TargetConfig)
	err = yaml.Unmarshal(b, result)
	return result, err
}

// readSFTPFile reads a file from a remote SFTP server
// unmarshals the content into a map[string]*types.TargetConfig
// and returns
func (f *fileLoader) readSFTPFile() (map[string]*types.TargetConfig, error) {
	parsedUrl, err := url.Parse(f.cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %v", err)
	}

	// Get user name and pass
	user := parsedUrl.User.Username()
	pass, _ := parsedUrl.User.Password()

	// Parse Host and Port
	host := parsedUrl.Host

	hostKey := f.getHostKey(host)

	f.logger.Printf("Connecting to %s ...", host)

	var auths []ssh.AuthMethod

	// Try to use $SSH_AUTH_SOCK which contains the path of the unix file socket that the sshd agent uses
	// for communication with other processes.
	if aconn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		auths = append(auths, ssh.PublicKeysCallback(agent.NewClient(aconn).Signers))
	}

	// Use password authentication if provided
	if pass != "" {
		auths = append(auths, ssh.Password(pass))
	}

	// Initialize client configuration
	config := ssh.ClientConfig{
		User: user,
		Auth: auths,
	}
	if hostKey != nil {
		config.HostKeyCallback = ssh.FixedHostKey(hostKey)
	} else {
		config.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	addr := fmt.Sprintf("%s:%d", host, 22)

	// Connect to server
	conn, err := ssh.Dial("tcp", addr, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to connecto to [%s]: %v", addr, err)
	}
	defer conn.Close()

	// Create new SFTP client
	sc, err := sftp.NewClient(conn)
	if err != nil {
		return nil, fmt.Errorf("unable to start SFTP subsystem: %v", err)
	}
	defer sc.Close()

	// open File
	file, err := sc.Open(parsedUrl.RequestURI())
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// stat file to get its size
	st, err := file.Stat()
	if err != nil {
		return nil, err
	}
	// create a []byte with length equal to the file size
	b := make([]byte, st.Size())
	// read the file
	_, err = file.Read(b)
	if err != nil {
		return nil, err
	}
	// unmarshal the received bytes and return the result
	result := make(map[string]*types.TargetConfig)
	err = yaml.Unmarshal(b, result)
	return result, err
}

// readLocalFile reads a file from the local file system,
// unmarshals the content into a map[string]*types.TargetConfig
// and returns
func (f *fileLoader) readLocalFile() (map[string]*types.TargetConfig, error) {
	_, err := os.Stat(f.cfg.Path)
	if err != nil {
		return nil, err
	}
	b, err := ioutil.ReadFile(f.cfg.Path)
	if err != nil {
		return nil, err
	}
	readTargets := make(map[string]*types.TargetConfig)
	switch filepath.Ext(f.cfg.Path) {
	case ".json":
		err = json.Unmarshal(b, &readTargets)
		if err != nil {
			return nil, err
		}
	case ".yaml", ".yml":
		err = yaml.Unmarshal(b, &readTargets)
		if err != nil {
			return nil, err
		}
	}
	for n, t := range readTargets {
		if t == nil {
			readTargets[n] = &types.TargetConfig{
				Name:    n,
				Address: n,
			}
			continue
		}
		if t.Name == "" {
			t.Name = n
		}
		if t.Address == "" {
			t.Address = n
		}
	}
	return readTargets, nil
}

func (f *fileLoader) diff(m map[string]*types.TargetConfig) *loaders.TargetOperation {
	result := loaders.Diff(f.lastTargets, m)
	for _, t := range result.Add {
		if _, ok := f.lastTargets[t.Name]; !ok {
			f.lastTargets[t.Name] = t
		}
	}
	for _, n := range result.Del {
		delete(f.lastTargets, n)
	}
	return result
}

// Get host key from local known hosts
func (f *fileLoader) getHostKey(host string) ssh.PublicKey {
	// parse OpenSSH known_hosts file
	// ssh or use ssh-keyscan to get initial key
	file, err := os.Open(filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts"))
	if err != nil {
		f.logger.Printf("failed to open known_hosts file: %v", err)
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var hostKey ssh.PublicKey
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), " ")
		if len(fields) != 3 {
			continue
		}
		if strings.Contains(fields[0], host) {
			var err error
			hostKey, _, _, _, err = ssh.ParseAuthorizedKey(scanner.Bytes())
			if err != nil {
				f.logger.Printf("failed to parse field %q: %v", string(scanner.Bytes()), err)
				return nil
			}
			break
		}
	}
	return hostKey
}
