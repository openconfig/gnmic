package file_loader

import (
	"bufio"
	"context"
	"crypto/tls"
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
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"gopkg.in/yaml.v2"
)

const (
	loggingPrefix   = "[file_loader] "
	watchInterval   = 30 * time.Second
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

// fileLoader implements the loaders.Loader interface.
// it reads a configured file (local, ftp, sftp, http) periodically, expects the file to contain
// a dictionnary of types.TargetConfig.
// It then adds new targets to gNMIc's targets and deletes the removes ones.
type fileLoader struct {
	cfg         *cfg
	lastTargets map[string]*types.TargetConfig
	logger      *log.Logger
}

type cfg struct {
	// path the the file, if remote,
	// must include the proper protocol prefix ftp://, sftp://, http://
	Path string `json:"path,omitempty" mapstructure:"path,omitempty"`
	// the interval at which the file will be re read to load new targets
	// or delete removed ones.
	Interval time.Duration `json:"interval,omitempty" mapstructure:"interval,omitempty"`
	// if true, registers fileLoader prometheus metrics with the provided
	// prometheus registry
	EnableMetrics bool `json:"enable-metrics,omitempty" mapstructure:"enable-metrics,omitempty"`
}

func (f *fileLoader) Init(ctx context.Context, cfg map[string]interface{}, logger *log.Logger, opts ...loaders.Option) error {
	err := loaders.DecodeConfig(cfg, f.cfg)
	if err != nil {
		return err
	}
	for _, o := range opts {
		o(f)
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

func (f *fileLoader) RegisterMetrics(reg *prometheus.Registry) {
	if !f.cfg.EnableMetrics && reg != nil {
		return
	}
	if err := registerMetrics(reg); err != nil {
		f.logger.Printf("failed to register metrics: %v", err)
	}
}

func (f *fileLoader) getTargets() (map[string]*types.TargetConfig, error) {
	var b []byte
	var err error
	// read file bytes based on the path prefix
	fileLoaderFileReadTotal.WithLabelValues(loaderType).Add(1)
	start := time.Now()
	switch {
	case strings.HasPrefix(f.cfg.Path, "https://"):
		fallthrough
	case strings.HasPrefix(f.cfg.Path, "http://"):
		b, err = f.readHTTPFile()
	case strings.HasPrefix(f.cfg.Path, "ftp://"):
		b, err = f.readFTPFile()
	case strings.HasPrefix(f.cfg.Path, "sftp://"):
		b, err = f.readSFTPFile()
	default:
		b, err = f.readLocalFile()
	}
	fileLoaderFileReadDuration.WithLabelValues(loaderType).Set(float64(time.Since(start).Nanoseconds()))
	if err != nil {
		fileLoaderFailedFileRead.WithLabelValues(loaderType, fmt.Sprintf("%v", err)).Add(1)
		return nil, err
	}
	result := make(map[string]*types.TargetConfig)
	// unmarshal the bytes into a map of targetConfigs
	err = yaml.Unmarshal(b, result)
	if err != nil {
		fileLoaderFailedFileRead.WithLabelValues(loaderType, fmt.Sprintf("%v", err)).Add(1)
		return nil, err
	}
	// properly initialize address and name if not set
	for n, t := range result {
		if t == nil && n != "" {
			result[n] = &types.TargetConfig{
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
	return result, nil
}

// readHTTPFile fetches a remote from from an HTTP server,
// the response body can be yaml or json bytes.
// it then unmarshal the received bytes into a map[string]*types.TargetConfig
// and returns
func (f *fileLoader) readHTTPFile() ([]byte, error) {
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
	return readBody, err
}

// readFTPFile reads a file from a remote FTP server
// unmarshals the content into a map[string]*types.TargetConfig
// and returns
func (f *fileLoader) readFTPFile() ([]byte, error) {
	parsedUrl, err := url.Parse(f.cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %v", err)
	}

	// Get user name and pass
	user := parsedUrl.User.Username()
	pass, _ := parsedUrl.User.Password()

	// Parse Host and Port
	host := parsedUrl.Host
	_, _, err = net.SplitHostPort(host)
	if err != nil {
		host = fmt.Sprintf("%s:%d", host, defaultFTPPort)
	}
	// connect to server
	conn, err := ftp.Dial(host, ftp.DialWithTimeout(f.cfg.Interval/2))
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
	return ioutil.ReadAll(r)
}

// readSFTPFile reads a file from a remote SFTP server
// unmarshals the content into a map[string]*types.TargetConfig
// and returns
func (f *fileLoader) readSFTPFile() ([]byte, error) {
	parsedUrl, err := url.Parse(f.cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %v", err)
	}

	// Get user name and pass
	user := parsedUrl.User.Username()
	pass, _ := parsedUrl.User.Password()

	// Parse Host and Port
	host := parsedUrl.Host
	_, _, err = net.SplitHostPort(host)
	if err != nil {
		host = fmt.Sprintf("%s:%d", host, defaultSFTPPort)
	}
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

	// Connect to server
	conn, err := ssh.Dial("tcp", host, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to connecto to [%s]: %v", host, err)
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
		return nil, fmt.Errorf("failed to open the remote file %q: %v", parsedUrl.RequestURI(), err)
	}
	defer file.Close()

	// stat file to get its size
	st, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return nil, fmt.Errorf("remote file %q is a directory", parsedUrl.RequestURI())
	}
	// create a []byte with length equal to the file size
	b := make([]byte, st.Size())
	// read the file
	_, err = file.Read(b)
	return b, err
}

// readLocalFile reads a file from the local file system,
// unmarshals the content into a map[string]*types.TargetConfig
// and returns
func (f *fileLoader) readLocalFile() ([]byte, error) {
	st, err := os.Stat(f.cfg.Path)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return nil, fmt.Errorf("%q is a directory", f.cfg.Path)
	}
	return ioutil.ReadFile(f.cfg.Path)
}

// diff compares the given map[string]*types.TargetConfig with the
// stored f.lastTargets and returns
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
	fileLoaderLoadedTargets.WithLabelValues(loaderType).Set(float64(len(result.Add)))
	fileLoaderDeletedTargets.WithLabelValues(loaderType).Set(float64(len(result.Del)))
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
