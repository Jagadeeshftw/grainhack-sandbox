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
