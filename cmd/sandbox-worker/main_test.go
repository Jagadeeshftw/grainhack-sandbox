package main

import (
	"io"
	"strings"
	"testing"
)

func TestParseFlags_RejectsConcurrencyBelowOne(t *testing.T) {
	for _, v := range []string{"0", "-1"} {
		t.Run(v, func(t *testing.T) {
			_, err := parseFlags([]string{"--concurrency", v}, io.Discard)
			if err == nil {
				t.Fatalf("--concurrency %s was accepted; a pool with no workers takes jobs and never runs them", v)
			}
			if !strings.Contains(err.Error(), "--concurrency") {
				t.Errorf("error %q does not name the flag", err)
			}
		})
	}
}

func TestParseFlags_AcceptsConcurrencyOfOne(t *testing.T) {
	cfg, err := parseFlags([]string{"--concurrency", "1"}, io.Discard)
	if err != nil {
		t.Fatalf("--concurrency 1 was rejected: %v", err)
	}
	if cfg.concurrency != 1 {
		t.Errorf("concurrency = %d, want 1", cfg.concurrency)
	}
}
