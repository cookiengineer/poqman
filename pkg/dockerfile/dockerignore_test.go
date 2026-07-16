package dockerfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cookiengineer/poqman/pkg/storage"
)

func TestLoadDockerIgnore_NoFile(t *testing.T) {
	patterns, err := LoadDockerIgnore(t.TempDir())
	if err != nil {
		t.Fatalf("LoadDockerIgnore: %v", err)
	}
	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns, got %d", len(patterns))
	}
}

func TestLoadDockerIgnore_Basic(t *testing.T) {
	tmp := t.TempDir()
	content := "# comment\n*.log\nnode_modules/\n.git\n!important.log\n"
	os.WriteFile(filepath.Join(tmp, ".dockerignore"), []byte(content), 0o644)

	patterns, err := LoadDockerIgnore(tmp)
	if err != nil {
		t.Fatalf("LoadDockerIgnore: %v", err)
	}
	if len(patterns) != 4 {
		t.Errorf("expected 4 patterns (comment filtered), got %d", len(patterns))
	}
}

func TestLoadDockerIgnore_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, ".dockerignore"), []byte(""), 0o644)

	patterns, err := LoadDockerIgnore(tmp)
	if err != nil {
		t.Fatalf("LoadDockerIgnore: %v", err)
	}
	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns, got %d", len(patterns))
	}
}

func TestShouldIgnore_Wildcard(t *testing.T) {
	patterns := []IgnorePattern{{Pattern: "*.log", Negate: false}}

	tests := []struct {
		path string
		want bool
	}{
		{"app.log", true},
		{"app.txt", false},
		{"logs/app.log", true},
		{"main.go", false},
	}
	for _, tt := range tests {
		if got := ShouldIgnore(tt.path, patterns); got != tt.want {
			t.Errorf("ShouldIgnore(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestShouldIgnore_Directory(t *testing.T) {
	patterns := []IgnorePattern{{Pattern: "node_modules/", Negate: false}}

	tests := []struct {
		path string
		want bool
	}{
		{"node_modules", true},
		{"node_modules/express", true},
		{"node_modules/express/index.js", true},
		{"src/node_modules", false},
		{"app.js", false},
	}
	for _, tt := range tests {
		if got := ShouldIgnore(tt.path, patterns); got != tt.want {
			t.Errorf("ShouldIgnore(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestShouldIgnore_Negate(t *testing.T) {
	patterns := []IgnorePattern{
		{Pattern: "*.log", Negate: false},
		{Pattern: "important.log", Negate: true},
	}
	if !ShouldIgnore("app.log", patterns) {
		t.Error("app.log should be ignored")
	}
	if ShouldIgnore("important.log", patterns) {
		t.Error("important.log should NOT be ignored (negation)")
	}
}

func TestShouldIgnore_GitDir(t *testing.T) {
	patterns := []IgnorePattern{
		{Pattern: ".git/", Negate: false},
	}
	if !ShouldIgnore(".git", patterns) {
		t.Error(".git should be ignored")
	}
	if !ShouldIgnore(".git/config", patterns) {
		t.Error(".git/config should be ignored")
	}
	if ShouldIgnore(".gitignore", patterns) {
		t.Error(".gitignore should NOT match .git/")
	}
}

func TestShouldIgnore_EmptyPatterns(t *testing.T) {
	if ShouldIgnore("anything.txt", nil) {
		t.Error("should not ignore when no patterns")
	}
	if ShouldIgnore("anything.txt", []IgnorePattern{}) {
		t.Error("should not ignore when empty patterns")
	}
}

func TestMatchPattern_Glob(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"*.go", "main.go", true},
		{"*.go", "main.py", false},
		{"*.go", "pkg/main.go", true},
		{"tmp/", "tmp", true},
		{"tmp/", "tmp/file", true},
		{"tmp/", "tmp/sub/file", true},
		{"build/", "build", true},
		{".git/", ".git", true},
		{".git/", ".gitignore", false},
		{"Dockerfile", "Dockerfile", true},
		{"Dockerfile", "src/Dockerfile", true},
	}
	for _, tt := range tests {
		if got := matchPattern(tt.pattern, tt.path); got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
		}
	}
}

func TestBuilderCopyWithIgnore(t *testing.T) {
	tmp := t.TempDir()
	contextDir := filepath.Join(tmp, "context")
	rootfsDir := filepath.Join(tmp, "rootfs")
	os.MkdirAll(rootfsDir, storage.DefaultPerms)
	os.MkdirAll(contextDir, storage.DefaultPerms)

	os.WriteFile(filepath.Join(contextDir, "app.log"), []byte("log"), 0o644)
	os.WriteFile(filepath.Join(contextDir, "app.go"), []byte("go"), 0o644)
	os.WriteFile(filepath.Join(contextDir, ".dockerignore"), []byte("*.log\n"), 0o644)

	ignorePatterns, _ := LoadDockerIgnore(contextDir)

	b := &Builder{
		contextPath:    contextDir,
		curRootfs:      rootfsDir,
		ignorePatterns: ignorePatterns,
	}

	b.handleCopy(&CopyInstruction{
		Sources:     []string{"."},
		Destination: "/app/",
	})

	if _, err := os.Stat(filepath.Join(rootfsDir, "app", "app.go")); os.IsNotExist(err) {
		t.Error("app.go should have been copied")
	}
	if _, err := os.Stat(filepath.Join(rootfsDir, "app", "app.log")); err == nil {
		t.Error("app.log should have been ignored by *.log pattern")
	}
}
