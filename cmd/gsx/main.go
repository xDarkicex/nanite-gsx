// Command gsx compiles .gsx template files into Go source and
// optionally watches a directory for changes (dev server).
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xDarkicex/nanite-gsx/internal/codegen"
	"github.com/xDarkicex/nanite-gsx/internal/parser"
)

func main() {
	compileCmd := flag.NewFlagSet("compile", flag.ExitOnError)
	compileDir := compileCmd.String("dir", "./views", "directory containing .gsx files")
	compileOut := compileCmd.String("out", "", "output directory (default: same as -dir)")

	watchCmd := flag.NewFlagSet("watch", flag.ExitOnError)
	watchDir := watchCmd.String("dir", "./views", "directory to watch for .gsx files")
	watchOut := watchCmd.String("out", "", "output directory (default: same as -dir)")

	if len(os.Args) < 2 {
		fmt.Println("usage: gsx <compile|watch> [-dir ./views] [-out ./views]")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "compile":
		compileCmd.Parse(os.Args[2:])
		if err := compileDirOnce(*compileDir, *compileOut); err != nil {
			log.Fatal(err)
		}
	case "watch":
		watchCmd.Parse(os.Args[2:])
		if err := watch(*watchDir, *watchOut); err != nil {
			log.Fatal(err)
		}
	default:
		fmt.Printf("unknown command %q\n", os.Args[1])
		os.Exit(1)
	}
}

func watch(dir, out string) error {
	// One-shot compile first so errors surface immediately.
	if err := compileDirOnce(dir, out); err != nil {
		return err
	}

	log.Printf("gsx watch: watching %s (Ctrl-C to stop)", dir)
	return watchLoop(dir, out)
}

// watchLoop polls dir every 100ms, recompiles any .gsx file
// whose modification time changed since the last pass. Simple,
// dependency-free, good enough for dev.
func watchLoop(dir, out string) error {
	lastMod := map[string]int64{}
	scan := func() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".gsx") {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			mod := info.ModTime().UnixNano()
			if lastMod[e.Name()] == mod {
				continue
			}
			lastMod[e.Name()] = mod
			srcPath := filepath.Join(dir, e.Name())
			if err := compileFile(srcPath, out); err != nil {
				log.Printf("gsx: %v", err) // don't crash the watcher
			}
		}
	}
	scan() // initial pass
	for {
		time.Sleep(100 * time.Millisecond)
		scan()
	}
}

// compileDirOnce compiles every .gsx file in dir into sibling
// _gsx.go files.
func compileDirOnce(dir, out string) error {
	if out == "" {
		out = dir
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".gsx") {
			continue
		}
		srcPath := filepath.Join(dir, e.Name())
		if err := compileFile(srcPath, out); err != nil {
			log.Printf("gsx: %v", err)
		}
	}
	return nil
}

// compileFile parses one .gsx file and writes the generated Go
// source next to it (or into out).
func compileFile(srcPath, out string) error {
	src, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("%s: read: %w", srcPath, err)
	}
	files, err := parser.Parse(src)
	if err != nil {
		return fmt.Errorf("%s: parse: %w", srcPath, err)
	}
	if len(files) == 0 {
		return fmt.Errorf("%s: no components found", srcPath)
	}
	generated, err := codegen.GenerateFile(files)
	if err != nil {
		return fmt.Errorf("%s: codegen: %w", srcPath, err)
	}
	base := strings.TrimSuffix(filepath.Base(srcPath), ".gsx")
	outPath := filepath.Join(out, base+"_gsx.go")
	if err := os.WriteFile(outPath, []byte(generated), 0o644); err != nil {
		return fmt.Errorf("%s: write: %w", outPath, err)
	}
	log.Printf("gsx: %s -> %s", srcPath, outPath)
	return nil
}
