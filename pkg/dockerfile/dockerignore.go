package dockerfile

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type IgnorePattern struct {
	Pattern  string
	Negate   bool
}

func LoadDockerIgnore(contextPath string) ([]IgnorePattern, error) {
	ignorePath := filepath.Join(contextPath, ".dockerignore")
	file, err := os.Open(ignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var patterns []IgnorePattern
	sc := bufio.NewScanner(file)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		negate := false
		if strings.HasPrefix(line, "!") {
			negate = true
			line = line[1:]
		}
		patterns = append(patterns, IgnorePattern{
			Pattern: line,
			Negate:  negate,
		})
	}
	return patterns, sc.Err()
}

func ShouldIgnore(path string, patterns []IgnorePattern) bool {
	ignored := false
	for _, p := range patterns {
		matched := matchPattern(p.Pattern, path)
		if !matched {
			continue
		}
		if p.Negate {
			ignored = false
		} else {
			ignored = true
		}
	}
	return ignored
}

func matchPattern(pattern, path string) bool {
	if strings.Contains(pattern, string(filepath.Separator)) || strings.HasSuffix(pattern, string(filepath.Separator)) {
		if strings.HasSuffix(pattern, string(filepath.Separator)) {
			pattern = strings.TrimSuffix(pattern, string(filepath.Separator))
		}
		return path == pattern || strings.HasPrefix(path, pattern+string(filepath.Separator))
	}

	if match, _ := filepath.Match(pattern, path); match {
		return true
	}
	if match, _ := filepath.Match(pattern, filepath.Base(path)); match {
		return true
	}
	return false
}
