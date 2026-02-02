package scanner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var SupportedExtensions = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".webp": true,
}

// ScanResults holds the found files and any errors encountered (permissions etc)
type ScanResults struct {
	Images []string
	Errors []error
}

// ScanFiles scans the given paths for images.
// If a path is a directory and recursive is true, it walks the directory.
// If a path is a glob pattern, it expands it.
func Scan(paths []string, recursive bool) (*ScanResults, error) {
	uniquePaths := make(map[string]bool)
	var errors []error

	for _, p := range paths {
		// Handle Glob
		matches, err := filepath.Glob(p)
		if err != nil {
			// If glob fails, assume it's a direct path (it might be a file with * in name, rare but possible, 
			// or just invalid glob syntax). Treat as literal path if glob failed?
			// filepath.Glob returns error only on BadPattern.
			errors = append(errors, fmt.Errorf("glob error %s: %w", p, err))
			continue
		}

		if matches == nil {
			// No matches, might be a direct file that hasn't been created yet? 
			// Or just a specific file path that Glob didn't match (e.g. absolute path without special chars? Glob matches those too).
			// If no match, check if exact file exists.
			if _, err := os.Stat(p); err == nil {
				matches = []string{p}
			}
		}

		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				errors = append(errors, err)
				continue
			}

			if info.IsDir() {
				if recursive {
					// Walk
					err := filepath.WalkDir(match, func(path string, d fs.DirEntry, err error) error {
						if err != nil {
							// Permission denied etc, log and continue
							// We don't want to stop the whole walk for one file
							return nil 
						}
						if !d.IsDir() && isSupported(path) {
							abs, err := filepath.Abs(path)
							if err == nil {
								uniquePaths[abs] = true
							}
						}
						return nil
					})
					if err != nil {
						errors = append(errors, fmt.Errorf("walk error %s: %w", match, err))
					}
				} else {
					// Directory but not recursive
					// The prompt says "Enter open directory" in TUI. 
					// For CLI "paths...", do we include only top level images?
				    // "Options: recursive" implies default might be non-recursive for folders? 
					// Let's assume just scan top level files if not recursive.
					entries, err := os.ReadDir(match)
					if err != nil {
						errors = append(errors, err)
						continue
					}
					for _, entry := range entries {
						if !entry.IsDir() && isSupported(entry.Name()) {
							fullPath := filepath.Join(match, entry.Name())
							abs, err := filepath.Abs(fullPath)
							if err == nil {
								uniquePaths[abs] = true
							}
						}
					}
				}
			} else {
				// File
				if isSupported(match) {
					abs, err := filepath.Abs(match)
					if err == nil {
						uniquePaths[abs] = true
					}
				}
			}
		}
	}

	images := make([]string, 0, len(uniquePaths))
	for p := range uniquePaths {
		images = append(images, p)
	}

	return &ScanResults{Images: images, Errors: errors}, nil
}

func isSupported(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return SupportedExtensions[ext]
}
