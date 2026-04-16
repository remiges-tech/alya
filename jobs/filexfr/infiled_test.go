package filexfr

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindFiles(t *testing.T) {
	logger := setupTestLogger(t)
	root := t.TempDir()

	mustWriteFile := func(relPath string) string {
		t.Helper()
		fullPath := filepath.Join(root, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(fullPath), err)
		}
		if err := os.WriteFile(fullPath, []byte("test"), 0o644); err != nil {
			t.Fatalf("write %s: %v", fullPath, err)
		}
		return fullPath
	}

	csvFile := mustWriteFile(filepath.Join("path", "to", "file.csv"))
	xlsxFile := mustWriteFile(filepath.Join("incoming", "data.xlsx"))
	datFile := mustWriteFile(filepath.Join("some", "path", "TXN20230615.dat"))
	nestedFile := mustWriteFile(filepath.Join("user1", "incoming", "txnbatch", "TXN_data.xlsx"))
	_ = mustWriteFile(filepath.Join("path", "to", "ignore.txt"))

	tests := []struct {
		name    string
		config  InfiledConfig
		pattern string
		want    []string
		wantErr bool
	}{
		{
			name:    "Match simple file extension",
			config:  InfiledConfig{WatchDirs: []string{filepath.Join(root, "path", "to")}},
			pattern: "*.csv",
			want:    []string{csvFile},
			wantErr: false,
		},
		{
			name:    "Match file in specific directory with leading slash",
			config:  InfiledConfig{WatchDirs: []string{root}},
			pattern: "/incoming/*.xlsx",
			want:    []string{xlsxFile},
			wantErr: false,
		},
		{
			name:    "Match file with prefix",
			config:  InfiledConfig{WatchDirs: []string{filepath.Join(root, "some", "path")}},
			pattern: "TXN*.dat",
			want:    []string{datFile},
			wantErr: false,
		},
		{
			name:    "Match file in nested directory",
			config:  InfiledConfig{WatchDirs: []string{root}},
			pattern: "/user1/incoming/txnbatch/TXN*.xlsx",
			want:    []string{nestedFile},
			wantErr: false,
		},
		{
			name:    "No match",
			config:  InfiledConfig{WatchDirs: []string{filepath.Join(root, "path", "to")}},
			pattern: "*.json",
			want:    []string{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &Infiled{
				config: tt.config,
				logger: logger,
			}
			got, err := i.findFiles(tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("findFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !equalPaths(got, tt.want) {
				t.Errorf("findFiles() = %v, want %v", got, tt.want)
			}
		})
	}
}

// equalPaths compares two slices of file paths, ignoring differences in path separators
func equalPaths(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if filepath.Clean(a[i]) != filepath.Clean(b[i]) {
			return false
		}
	}
	return true
}
