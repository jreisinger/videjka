// videjka finds video files with human-created names inside a [OneDrive] folder.
package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var videoExt = map[string]bool{
	".mp4":  true,
	".mov":  true,
	".avi":  true,
	".mkv":  true,
	".m4v":  true,
	".wmv":  true,
	".flv":  true,
	".webm": true,
	".mpg":  true,
	".mpeg": true,
}

var machinePatterns = []*regexp.Regexp{
	// Only digits, spaces, dots, underscores, hyphens
	regexp.MustCompile(`^[\d ._\\-]+$`),
	// Camera prefixes
	regexp.MustCompile(`(?i)^(IMG|MOV|MVI|DSC|DCIM|VID)[_-]`),
	// Camera suffix
	regexp.MustCompile(`_iOS$`),
	// Long numbers
	regexp.MustCompile(`\d{10,}`),
	// Like P4300021 ...
	regexp.MustCompile(`(?i)^P\d+`),
}

func isHuman(filename string) bool {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	for _, re := range machinePatterns {
		if re.MatchString(name) {
			return false
		}
	}
	return true
}

// copy copies src file to dest file creating any necessary
// parents for dest path.
func copy(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// joinWithSubdirs joins dstDir with srcPath retaining levels of subdirectories
// from srcPath.
func joinWithSubdirs(dstDir, srcPath string, levels int) string {
	if levels < 0 {
		return filepath.Join(dstDir, srcPath)
	}

	src := filepath.Dir(srcPath)
	if src == "." {
		return filepath.Join(dstDir, filepath.Base(srcPath))
	}

	parts := strings.Split(filepath.Clean(src), string(os.PathSeparator))
	if levels > len(parts) {
		levels = len(parts)
	}
	keep := parts[len(parts)-levels:]

	return filepath.Join(dstDir, filepath.Join(keep...), filepath.Base(srcPath))
}

func main() {
	defaultDir := filepath.Join(os.Getenv("HOME"), "OneDrive") + string(os.PathSeparator)
	dir := flag.String("dir", defaultDir, "directory to search")
	cpDir := flag.String("cp", "", "copy the found files into `directory`")
	levels := flag.Int("L", 0, "retain `n` directory levels")
	flag.Parse()

	var errors []string

	err := filepath.WalkDir(*dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip: %v\n", err)
			return nil
		}

		if d.IsDir() || !videoExt[strings.ToLower(filepath.Ext(d.Name()))] || !isHuman(d.Name()) {
			return nil
		}

		rel, err := filepath.Rel(*dir, path)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", path, err))
			return nil
		}

		dest := joinWithSubdirs(*cpDir, rel, *levels)

		if len(*cpDir) <= 0 {
			fmt.Printf("%q -> %q\n", path, dest)
			return nil
		}

		if err := copy(path, dest); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", path, err))
		}

		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(errors) > 0 {
		fmt.Fprintln(os.Stderr, "errors:")
		for _, e := range errors {
			fmt.Fprintln(os.Stderr, " ", e)
		}
		os.Exit(1)
	}
}
