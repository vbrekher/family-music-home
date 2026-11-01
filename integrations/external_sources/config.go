// Package externalsources validates operator-managed, read-only remote music sources.
// Runtime providers resolve only these administrator-approved source IDs.
package externalsources

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/url"
	"regexp"
	"strings"
)

// Source stores no credential values. SecretEnv names an existing server-side secret.
type Source struct {
	SocksProxyEnv   string   `json:"socks_proxy_env,omitempty"`
	AllowedNetworks []string `json:"allowed_networks,omitempty"`
	ID              string   `json:"id"`
	Type            string   `json:"type"`
	Endpoint        string   `json:"endpoint"`
	Root            string   `json:"root"`
	Bucket          string   `json:"bucket,omitempty"`
	Region          string   `json:"region,omitempty"`
	SecretEnv       string   `json:"secret_env"`
	HostKeySHA256   string   `json:"host_key_sha256,omitempty"`
	ReadOnly        bool     `json:"read_only"`
	Enabled         bool     `json:"enabled"`
}
type Config struct {
	Sources []Source `json:"sources"`
}

var (
	idPattern          = regexp.MustCompile(`^[a-z][a-z0-9-]{1,47}$`)
	secretPattern      = regexp.MustCompile(`^NAVIDROME_SOURCE_[A-Z0-9_]+$`)
	bucketPattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)
	fingerprintPattern = regexp.MustCompile(`^SHA256:[A-Za-z0-9+/]{43}=?$`)
)

func Parse(data []byte) (Config, error) {
	if len(data) > 1<<20 {
		return Config{}, errors.New("external sources config too large")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	var c Config
	if err := d.Decode(&c); err != nil {
		return Config{}, fmt.Errorf("invalid external sources JSON: %w", err)
	}
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF {
		return Config{}, errors.New("external sources config has trailing data")
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}
func (c Config) Validate() error {
	if len(c.Sources) > 32 {
		return errors.New("too many external sources")
	}
	seen := map[string]bool{}
	for _, s := range c.Sources {
		if !idPattern.MatchString(s.ID) {
			return errors.New("invalid external source id")
		}
		if seen[s.ID] {
			return fmt.Errorf("duplicate external source id %q", s.ID)
		}
		seen[s.ID] = true
		if s.SocksProxyEnv != "" && !secretPattern.MatchString(s.SocksProxyEnv) {
			return errors.New("invalid private proxy reference")
		}
		for _, cidr := range s.AllowedNetworks {
			if _, _, e := net.ParseCIDR(cidr); e != nil {
				return errors.New("invalid approved network CIDR")
			}
		}
		if !s.ReadOnly {
			return fmt.Errorf("source %q must be read-only", s.ID)
		}
		if !secretPattern.MatchString(s.SecretEnv) {
			return fmt.Errorf("source %q requires a server-side secret reference", s.ID)
		}
		if s.Root != "." && (!fs.ValidPath(s.Root) || strings.Contains(s.Root, "\\")) {
			return fmt.Errorf("source %q has invalid relative root", s.ID)
		}
		u, err := url.Parse(s.Endpoint)
		if err != nil || u == nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || u.Path != "" || u.RawPath != "" || u.ForceQuery {
			return fmt.Errorf("source %q has invalid origin (credentials, paths and queries forbidden)", s.ID)
		}
		host := u.Hostname()
		if strings.ContainsAny(host, " \t\r\n") || strings.HasSuffix(host, ".") {
			return fmt.Errorf("source %q has invalid hostname", s.ID)
		}
		if port := u.Port(); port != "" {
			n, e := net.LookupPort("tcp", port)
			if e != nil || n <= 0 || n > 65535 {
				return fmt.Errorf("source %q has invalid port", s.ID)
			}
		}
		switch s.Type {
		case "s3":
			if u.Scheme != "https" || !bucketPattern.MatchString(s.Bucket) || s.HostKeySHA256 != "" || s.Region == "" || strings.ContainsAny(s.Region, "/:@? ") {
				return fmt.Errorf("source %q has invalid S3 configuration", s.ID)
			}
		case "webdav":
			if u.Scheme != "https" || s.Bucket != "" || s.Region != "" || s.HostKeySHA256 != "" {
				return fmt.Errorf("source %q requires HTTPS WebDAV", s.ID)
			}
		case "sftp":
			if u.Scheme != "sftp" || !fingerprintPattern.MatchString(s.HostKeySHA256) || s.Bucket != "" || s.Region != "" {
				return fmt.Errorf("source %q requires SFTP with a pinned host key", s.ID)
			}
		default:
			return fmt.Errorf("source %q has unsupported provider", s.ID)
		}
	}
	return nil
}
