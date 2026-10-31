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

// copy copies src file to dst file creating any necessary parents for dst path.
// Copying is skipped if dst already exists and has the same size as src.
func copy(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	dstInfo, err := os.Stat(dst)
	if err == nil && dstInfo.Size() == srcInfo.Size() {
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst) // creates or truncates
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
	if levels <= 0 {
		return filepath.Join(dstPath, filepath.Base(srcPath))
	}

	srcDir := filepath.Dir(srcPath)
	srcBase := filepath.Base(srcPath)

	parts := strings.Split(filepath.Clean(srcDir), string(os.PathSeparator))
	if levels > len(parts) {
		levels = len(parts)
	}
	keep := parts[len(parts)-levels:]

	return filepath.Join(dstPath, filepath.Join(keep...), srcBase)
}

// getVideos is to be used by filepath.Walkdir. The returned function sends
// paths of video files with human names down the channel.
func getVideos(ctx context.Context, in chan<- string) fs.WalkDirFunc {
	return func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err != nil {
			return err
		}

		if d.IsDir() || !isVideo(d.Name()) || !isHuman(d.Name()) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case in <- path:
			return nil
		}
	}
}

func main() {
	log.SetFlags(0)
	log.SetPrefix(filepath.Base(os.Args[0]) + ": ")

	levels := flag.Int("L", 0, "retain `n` directory levels from source path")
	cpDir := flag.String("cp", "", "copy the found videos into `dest`")
	defaultDir := filepath.Join(os.Getenv("HOME"), "OneDrive") + string(os.PathSeparator)
	root := flag.String("p", defaultDir, "path to search for videos")
	flag.Parse()

	// Handle Ctrl-C gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Printf("shutting down ...")
		stop() // Restore default behavior so a second signal forces exit.
	}()

	wg := sync.WaitGroup{}
	videos := make(chan string)

	// Search videos.
	wg.Go(func() {
		defer close(videos)
		err := filepath.WalkDir(*root, getVideos(ctx, videos))
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Fatal(err)
		}
	})

	// Copy or just print the found videos.
	for range 5 {
		wg.Go(func() {
			for src := range videos {
				if len(*cpDir) > 0 {
					dst := joinWithDirs(*cpDir, src, *levels)
					err := copy(src, dst)
					if err != nil {
						log.Print(err)
						continue
					}
					fmt.Printf("%q -> %q\n", src, dst)
				} else {
					fmt.Printf("%q\n", src)
				}
			}
		})
	}

	wg.Wait()
}
