package config

import (
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	c := Default()

	if c.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", c.Server.Port)
	}
	if c.Engine.WorkerCount != 4 {
		t.Errorf("expected 4 workers, got %d", c.Engine.WorkerCount)
	}
	if c.Engine.MaxLoops != 100 {
		t.Errorf("expected max loops 100, got %d", c.Engine.MaxLoops)
	}
	if c.Log.Level != "info" {
		t.Errorf("expected log level info, got %s", c.Log.Level)
	}
	if c.Log.Format != "json" {
		t.Errorf("expected log format json, got %s", c.Log.Format)
	}
	if !c.Audit.Enabled {
		t.Error("expected audit enabled by default")
	}
	if c.Audit.Dir != "./data/audit" {
		t.Errorf("expected audit dir ./data/audit, got %s", c.Audit.Dir)
	}
}

func TestValidate_OK(t *testing.T) {
	c := Default()
	if err := c.Validate(); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	c := Default()
	c.Server.Port = 0
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for port 0")
	}

	c.Server.Port = 70000
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for port 70000")
	}
}

func TestValidate_InvalidWorkerCount(t *testing.T) {
	c := Default()
	c.Engine.WorkerCount = 0
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for worker count 0")
	}
}

func TestValidate_InvalidMaxLoops(t *testing.T) {
	c := Default()
	c.Engine.MaxLoops = 0
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for max loops 0")
	}
}

func TestValidate_InvalidTimeout(t *testing.T) {
	c := Default()
	c.Engine.ExecutionTimeout = 500 * time.Millisecond
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for timeout < 1s")
	}
}

func TestValidate_EmptyAuditDir(t *testing.T) {
	c := Default()
	c.Audit.Enabled = true
	c.Audit.Dir = ""
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for empty audit dir when enabled")
	}
}

func TestAuditDefaults(t *testing.T) {
	c := Default()
	if c.Audit.Enabled != true {
		t.Error("expected audit enabled by default")
	}
	if c.Audit.Dir == "" {
		t.Error("expected non-empty audit dir")
	}
}
