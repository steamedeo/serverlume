package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSSHHosts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	contents := `
Host web-01
    HostName 10.0.1.11
    Port 2222

Host db-primary
    HostName 10.0.3.5
    User dbadmin

Host 10.0.*
    User skipme

Host *
    User defaultuser
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	hosts, err := loadSSHHosts(path)
	if err != nil {
		t.Fatalf("loadSSHHosts: %v", err)
	}
	if len(hosts) != 2 {
		t.Fatalf("got %d hosts, want 2: %+v", len(hosts), hosts)
	}

	byAlias := make(map[string]sshHost)
	for _, h := range hosts {
		byAlias[h.Alias] = h
	}

	web, ok := byAlias["web-01"]
	if !ok {
		t.Fatalf("missing web-01 in %+v", hosts)
	}
	if web.HostName != "10.0.1.11" || web.Port != "2222" || web.User != "defaultuser" {
		t.Errorf("web-01 = %+v, want HostName=10.0.1.11 Port=2222 User=defaultuser (inherited from Host *)", web)
	}

	db, ok := byAlias["db-primary"]
	if !ok {
		t.Fatalf("missing db-primary in %+v", hosts)
	}
	if db.HostName != "10.0.3.5" || db.User != "dbadmin" {
		t.Errorf("db-primary = %+v, want HostName=10.0.3.5 User=dbadmin", db)
	}
}

func TestLoadSSHHostsMissingFile(t *testing.T) {
	if _, err := loadSSHHosts(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Fatal("expected an error for a missing config file, got nil")
	}
}

func TestDiscoverSSHFleetSeedsMetrics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	contents := "Host api-01\n    HostName 10.0.2.10\n    User svc\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	hosts, err := loadSSHHosts(path)
	if err != nil {
		t.Fatalf("loadSSHHosts: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("got %d hosts, want 1", len(hosts))
	}

	s := seedSSHServer(hosts[0])
	if s.name != "api-01" || s.ip != "10.0.2.10" {
		t.Errorf("seedSSHServer = %+v, want name=api-01 ip=10.0.2.10", s)
	}
	if len(s.cpuHist) == 0 || len(s.memHist) == 0 || len(s.diskHist) == 0 || len(s.netHist) == 0 {
		t.Errorf("seedSSHServer left empty history: %+v", s)
	}
}
