package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
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

func isVideo(filename string) bool {
	ext := filepath.Ext(filename)
	return videoExt[strings.ToLower(ext)]
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
	// Like P4300021
	regexp.MustCompile(`(?i)^P\d+`),
	// Like 20231202_141942000_iOS
	regexp.MustCompile(`_iOS$`),
	// Like bfb1adf7-8be1-453f-9c03-326b46ff0e88
	regexp.MustCompile(`(?i)[a-f0-9]{12,}`),
	// Like MOV02832
	regexp.MustCompile(`^MOV\d+$`),
	// Like foto 136
	regexp.MustCompile(`^foto\s*\d+$`),
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

type video struct {
	path string
	dest string
}

// copy copies src file to dest file creating any necessary
// parents for dest path.
func copy(src, dest string) error {
	err := os.MkdirAll(filepath.Dir(dest), 0755)
	if err != nil {
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

// joinWithDirs joins dstPath with srcPath retaining levels of directories from
// srcPath.
func joinWithDirs(dstPath, srcPath string, levels int) string {
	if levels < 0 {
		return filepath.Join(dstPath, srcPath)
	}

	src := filepath.Dir(srcPath)
	if src == "." {
		return filepath.Join(dstPath, filepath.Base(srcPath))
	}

	parts := strings.Split(filepath.Clean(src), string(os.PathSeparator))
	if levels > len(parts) {
		levels = len(parts)
	}
	keep := parts[len(parts)-levels:]

	return filepath.Join(dstPath, filepath.Join(keep...), filepath.Base(srcPath))
}

func main() {
	log.SetFlags(0)
	log.SetPrefix(filepath.Base(os.Args[0]) + ": ")

	defaultDir := filepath.Join(os.Getenv("HOME"), "OneDrive") + string(os.PathSeparator)
	root := flag.String("path", defaultDir, "path to search")
	cpDir := flag.String("cp", "", "copy the found files into `path`")
	levels := flag.Int("L", 0, "retain `n` directory levels from source path")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Printf("received signal, waiting for active copies to finish ...")
		stop() // Restore default behavior so a second signal forces exit.
	}()

	videos := make(chan video)
	wg := sync.WaitGroup{}

	wg.Go(func() {
		defer close(videos)
		err := filepath.WalkDir(*root, getVideos(ctx, videos))
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Fatal(err)
		}
	})

	for range 5 {
		wg.Go(func() {
			for v := range videos {
				dest := joinWithDirs(*cpDir, v.path, *levels)
				if len(*cpDir) > 0 {
					err := copy(v.path, dest)
					if err != nil {
						log.Print(err)
						continue
					}
				}
				fmt.Printf("%q -> %q\n", v.path, dest)
			}
		})
	}

	wg.Wait()
}

func getVideos(ctx context.Context, in chan<- video) fs.WalkDirFunc {
	return func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err != nil {
			log.Print(err)
			return nil
		}

		if d.IsDir() {
			return nil
		}

		if !isVideo(d.Name()) {
			return nil
		}

		if !isHuman(d.Name()) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case in <- video{path: path}:
			return nil
		}
	}
}
