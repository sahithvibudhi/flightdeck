package system

import (
	"testing"
)

func TestDetect_ReturnsAllRuntimes(t *testing.T) {
	info := Detect()

	if info.OS == "" {
		t.Error("expected non-empty OS")
	}
	if info.Arch == "" {
		t.Error("expected non-empty Arch")
	}

	expected := []string{"Git", "Node.js", "Python", "Go", "Docker", "Bun", "Deno"}
	if len(info.Runtimes) != len(expected) {
		t.Fatalf("expected %d runtimes, got %d", len(expected), len(info.Runtimes))
	}

	for i, name := range expected {
		if info.Runtimes[i].Name != name {
			t.Errorf("runtime[%d]: expected %s, got %s", i, name, info.Runtimes[i].Name)
		}
	}
}

func TestDetect_InstalledHaveVersions(t *testing.T) {
	info := Detect()

	for _, r := range info.Runtimes {
		if r.Installed && r.Version == "" {
			t.Errorf("%s is installed but has empty version", r.Name)
		}
		if !r.Installed && r.Version != "" {
			t.Errorf("%s is not installed but has version %q", r.Name, r.Version)
		}
	}
}

func TestTrimmers(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"Git", "git version 2.43.0", "2.43.0"},
		{"Node.js", "v20.11.0", "20.11.0"},
		{"Python", "Python 3.12.1", "3.12.1"},
		{"Docker", "Docker version 27.1.2, build abc123", "27.1.2"},
		{"Docker", "Docker version 24.0.0", "24.0.0"},
	}

	trimmers := map[string]func(string) string{}
	for _, c := range checks {
		if c.trimmer != nil {
			trimmers[c.name] = c.trimmer
		}
	}

	for _, tc := range cases {
		t.Run(tc.name+"_"+tc.input, func(t *testing.T) {
			fn, ok := trimmers[tc.name]
			if !ok {
				t.Skipf("no trimmer for %s", tc.name)
			}
			got := fn(tc.input)
			if got != tc.want {
				t.Errorf("trimmer(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
