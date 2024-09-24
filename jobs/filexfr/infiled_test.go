package filexfr

import (
	"path/filepath"
	"testing"
)

func TestFindFiles(t *testing.T) {
	logger := setupTestLogger(t)

	tests := []struct {
		name    string
		config  InfiledConfig
		pattern string
		want    []string
		wantErr bool
	}{
		{
			name: "Match simple file extension",
			config: InfiledConfig{
				WatchDirs: []string{"/path/to"},
			},
			pattern: "*.csv",
			want:    []string{"/path/to/file.csv"},
			wantErr: false,
		},
		{
			name: "Match file in specific directory",
			config: InfiledConfig{
				WatchDirs: []string{"/"},
			},
			pattern: "/incoming/*.xlsx",
			want:    []string{"/incoming/data.xlsx"},
			wantErr: false,
		},
		{
			name: "Match file with prefix",
			config: InfiledConfig{
				WatchDirs: []string{"/some/path"},
			},
			pattern: "TXN*.dat",
			want:    []string{"/some/path/TXN20230615.dat"},
			wantErr: false,
		},
		{
			name: "Match file in nested directory",
			config: InfiledConfig{
				WatchDirs: []string{"/user1"},
			},
			pattern: "/*/incoming/txnbatch/TXN*.xlsx",
			want:    []string{"/user1/incoming/txnbatch/TXN_data.xlsx"},
			wantErr: false,
		},
		{
			name: "No match",
			config: InfiledConfig{
				WatchDirs: []string{"/path/to"},
			},
			pattern: "*.csv",
			want:    []string{},
			wantErr: false,
		},
		// Add more test cases as needed
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
