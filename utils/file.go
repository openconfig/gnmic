// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/jlaffaye/ftp"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	defaultFTPPort  = 21
	defaultSFTPPort = 22
)

// ReadFile reads a local or remote file and returns the read bytes,
// the location of the file is determined based on its prefix,
// http(s), (s)ftp prefixes are supported.
// no prefix means the file is local. `-` means stdin.
func ReadFile(ctx context.Context, path string) ([]byte, error) {
	// read file bytes based on the path prefix
	switch {
	case strings.HasPrefix(path, "https://"):
		return readHTTPFile(ctx, path)
	case strings.HasPrefix(path, "http://"):
		return readHTTPFile(ctx, path)
	case strings.HasPrefix(path, "ftp://"):
		return readFTPFile(ctx, path)
	case strings.HasPrefix(path, "sftp://"):
		return readSFTPFile(ctx, path, false)
	default:
		return readLocalFile(ctx, path)
	}
}

// readHTTPFile fetches a remote from from an HTTP server,
// the response body can be yaml or json bytes.
// it then unmarshal the received bytes into a map[string]*types.TargetConfig
// and returns
func readHTTPFile(ctx context.Context, path string) ([]byte, error) {
	_, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	client := new(http.Client)
	if strings.HasPrefix(path, "https://") {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, new(bytes.Buffer))
	if err != nil {
		return nil, err
	}
	r, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected HTTP status code %d, GET from %s", r.StatusCode, path)
	}
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}

// readFTPFile reads a file from a remote FTP server
// unmarshals the content into a map[string]*types.TargetConfig
// and returns
func readFTPFile(ctx context.Context, path string) ([]byte, error) {
	parsedUrl, err := url.Parse(path)
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

	conn, err := ftp.Dial(host, ftp.DialWithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to [%s]: %v", host, err)
	}

	err = conn.Login(user, pass)
	if err != nil {
		return nil, fmt.Errorf("failed to login to [%s]: %v", host, err)
	}

	r, err := conn.Retr(parsedUrl.RequestURI())
	if err != nil {
		return nil, fmt.Errorf("failed to read remote file %q: %v", parsedUrl.RequestURI(), err)
	}
	defer r.Close()
	return io.ReadAll(r)
}

// readSFTPFile reads a file from a remote SFTP server
// unmarshals the content into a map[string]*types.TargetConfig
// and returns
func readSFTPFile(_ context.Context, path string, checkHostKey bool) ([]byte, error) {
	parsedUrl, err := url.Parse(path)
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

	// if checkHostKey is set, try loading the know_hosts file
	if checkHostKey {
		knownHostsFile := filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts")
		// check ~/.ssh/known_hosts existence
		if !FileExists(knownHostsFile) {
			return nil, fmt.Errorf("known_hosts file %s does not exist", knownHostsFile)
		}

		// load the known_hosts file retrieving an ssh.HostKeyCallback
		config.HostKeyCallback, err = knownhosts.New(knownHostsFile)
		if err != nil {
			return nil, err
		}
	} else {
		// use the use the InsecureIgnoreHostKey implementation
		config.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	// Connect to server
	conn, err := ssh.Dial("tcp", host, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to [%s]: %v", host, err)
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
func readLocalFile(ctx context.Context, path string) ([]byte, error) {
	// read from stdin
	if path == "-" {
		return readFromStdin(ctx)
	}

	// local file
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return nil, fmt.Errorf("%q is a directory", path)
	}
	data := make([]byte, st.Size())

	rd := bufio.NewReader(f)
	_, err = rd.Read(data)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return data, nil
}

// read bytes from stdin
func readFromStdin(ctx context.Context) ([]byte, error) {
	// read from stdin
	data := make([]byte, 0, 128)
	rd := bufio.NewReader(os.Stdin)
	buf := make([]byte, 128)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			n, err := rd.Read(buf)
			if err == io.EOF {
				data = append(data, buf[:n]...)
				return data, nil
			}
			if err != nil {
				return nil, err
			}
			data = append(data, buf[:n]...)
		}
	}
}

// FileExists returns true if a file referenced by filename exists & accessible.
func FileExists(filename string) bool {
	f, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !f.IsDir()
}
