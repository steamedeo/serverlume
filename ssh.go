// Package main: SSH fleet discovery.
//
// serverlume populates its server list from the local OpenSSH client
// configuration (~/.ssh/config) rather than a hardcoded inventory, so it
// works against whatever hosts the machine it runs on already knows how to
// reach — on Linux, macOS, or Windows (the built-in Windows OpenSSH client
// honors the same file, under the user's profile directory).
package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kevinburke/ssh_config"
)

// sshHost is one concrete (non-wildcard) Host alias discovered in an SSH
// client config file.
type sshHost struct {
	Alias    string
	HostName string
	User     string
	Port     string
}

// defaultSSHConfigPath returns the conventional per-user OpenSSH client
// config location: "<home>/.ssh/config". The relative path is identical
// across Linux, macOS, and Windows; only the home directory resolution
// differs, which os.UserHomeDir handles per platform.
func defaultSSHConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "config"), nil
}

// loadSSHHosts reads and parses an OpenSSH client config file, returning one
// entry per concrete Host alias it defines. Patterns containing wildcards
// (e.g. "Host *" or "Host 10.0.*") are skipped, since they describe defaults
// to apply to other hosts rather than a specific, monitorable machine.
//
// Values are resolved through the config's own alias-matching rules via
// Config.Get, so settings inherited from a broader pattern (e.g. a shared
// "User" set under "Host *") are picked up correctly. Hosts defined only
// inside a file pulled in via an "Include" directive are not enumerated,
// since the underlying parser does not expose included files' Host blocks —
// only their key/value lookups for an already-known alias.
func loadSSHHosts(path string) ([]sshHost, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg, err := ssh_config.Decode(f)
	if err != nil {
		return nil, err
	}

	var hosts []sshHost
	seen := make(map[string]bool)
	for _, block := range cfg.Hosts {
		for _, pattern := range block.Patterns {
			alias := pattern.String()
			if alias == "" || strings.ContainsAny(alias, "*?!") || seen[alias] {
				continue
			}
			seen[alias] = true

			hostName, _ := cfg.Get(alias, "HostName")
			if hostName == "" {
				hostName = alias
			}
			user, _ := cfg.Get(alias, "User")
			port, _ := cfg.Get(alias, "Port")

			hosts = append(hosts, sshHost{Alias: alias, HostName: hostName, User: user, Port: port})
		}
	}
	return hosts, nil
}

// discoverSSHFleet builds the server list from the local SSH client config.
// It returns nil (never an error) when no config file exists or it defines
// no concrete hosts, so callers can fall back to demo data without needing
// to distinguish "missing config" from "empty config".
func discoverSSHFleet() []server {
	path, err := defaultSSHConfigPath()
	if err != nil {
		return nil
	}
	hosts, err := loadSSHHosts(path)
	if err != nil || len(hosts) == 0 {
		return nil
	}

	srv := make([]server, 0, len(hosts))
	for _, h := range hosts {
		srv = append(srv, seedSSHServer(h))
	}
	return srv
}
