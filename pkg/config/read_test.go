package config

import (
	"reflect"
	"testing"
)

func TestToEnv(t *testing.T) {
	obj := struct {
		PortNumber int    `json:"portNumber"`
		Enabled    bool   `json:"enabled"`
		Name       string `json:"name"`
	}{
		PortNumber: 8443,
		Enabled:    true,
		Name:       "harvester",
	}

	envs, err := ToEnv("PREFIX_", &obj)
	if err != nil {
		t.Fatalf("ToEnv() error = %v", err)
	}

	want := map[string]struct{}{
		"PREFIX_PORT_NUMBER=8443": {},
		"PREFIX_ENABLED=true":     {},
		"PREFIX_NAME=harvester":   {},
	}

	got := make(map[string]struct{})
	for _, env := range envs {
		got[env] = struct{}{}
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got = %v, want %v", got, want)
	}
}
