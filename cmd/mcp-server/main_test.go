package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWorkspace(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a-file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatal(err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"empty means the working directory", "", cwd, false},
		{"dot means the working directory", ".", cwd, false},
		{"absolute directory", sub, sub, false},
		{"trailing slash is cleaned", sub + "/", sub, false},
		{"unclean path is cleaned", filepath.Join(sub, "..", "sub"), sub, false},
		{"a file is refused", file, "", true},
		{"a missing directory is refused", filepath.Join(dir, "nope"), "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveWorkspace(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveWorkspace(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("resolveWorkspace(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if !filepath.IsAbs(got) {
				t.Errorf("result %q is not absolute", got)
			}
		})
	}
}

func TestEnvHelpers(t *testing.T) {
	t.Run("envOrDefault", func(t *testing.T) {
		t.Setenv("CAGED_TEST_STR", "set")
		if got := envOrDefault("CAGED_TEST_STR", "fallback"); got != "set" {
			t.Errorf("got %q", got)
		}
		if got := envOrDefault("CAGED_TEST_UNSET", "fallback"); got != "fallback" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("envIntOrDefault", func(t *testing.T) {
		t.Setenv("CAGED_TEST_INT", "8080")
		if got := envIntOrDefault("CAGED_TEST_INT", 9090); got != 8080 {
			t.Errorf("got %d", got)
		}
		t.Setenv("CAGED_TEST_INT", "not a number")
		if got := envIntOrDefault("CAGED_TEST_INT", 9090); got != 9090 {
			t.Errorf("garbage should fall back, got %d", got)
		}
	})
	t.Run("envBoolOrDefault", func(t *testing.T) {
		for _, tc := range []struct {
			value string
			def   bool
			want  bool
		}{
			{"true", false, true},
			{"1", false, true},
			{"false", true, false},
			{"", true, true},
			{"", false, false},
		} {
			t.Setenv("CAGED_TEST_BOOL", tc.value)
			if got := envBoolOrDefault("CAGED_TEST_BOOL", tc.def); got != tc.want {
				t.Errorf("envBoolOrDefault(%q, %v) = %v, want %v", tc.value, tc.def, got, tc.want)
			}
		}
	})
}
