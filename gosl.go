// Copyright (c) 2022, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// copied and heavily edited from go src/cmd/gofmt/gofmt.go:

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/goki/gosl/slprint"
)

// flags
var (
	outDir  = flag.String("out", "shaders", "output directory for shader code, relative to where gosl is invoked")
	keepTmp = flag.Bool("keep", false, "keep temporary converted versions of the source files, for debugging")
)

// Keep these in sync with go/format/format.go.
const (
	tabWidth    = 8
	printerMode = slprint.UseSpaces | slprint.TabIndent | printerNormalizeNumbers

	// printerNormalizeNumbers means to canonicalize number literal prefixes
	// and exponents while printing. See https://golang.org/doc/go1.13#gosl.
	//
	// This value is defined in go/printer specifically for go/format and cmd/gosl.
	printerNormalizeNumbers = 1 << 30
)

// fdSem guards the number of concurrently-open file descriptors.
//
// For now, this is arbitrarily set to 200, based on the observation that many
// platforms default to a kernel limit of 256. Ideally, perhaps we should derive
// it from rlimit on platforms that support that system call.
//
// File descriptors opened from outside of this package are not tracked,
// so this limit may be approximate.
var fdSem = make(chan bool, 200)

var (
	outFiles   []string            // list of all output files saved
	filesProcd = map[string]bool{} // prevent redundancies
	parserMode parser.Mode
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: gosl [flags] [path ...]\n")
	flag.PrintDefaults()
}

func initParserMode() {
	parserMode = parser.ParseComments
}

func isGoFile(f fs.DirEntry) bool {
	// ignore non-Go files
	name := f.Name()
	return !strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".go") && !f.IsDir()
}

// returns name of output file
func processFile(filename string, info fs.FileInfo) (string, error) {
	if _, exists := filesProcd[filename]; exists {
		return "", nil
	}
	filesProcd[filename] = true

	src, err := readFile(filename, info)
	if err != nil {
		return "", err
	}

	fileSet := token.NewFileSet()
	fragmentOk := false
	if info == nil {
		// If we are formatting stdin, we accept a program fragment in lieu of a
		// complete source file.
		fragmentOk = true
	}
	file, sourceAdj, indentAdj, err := parse(fileSet, filename, src, fragmentOk)
	if err != nil {
		return "", err
	}

	ast.SortImports(fileSet, file)

	res, err := format(fileSet, file, sourceAdj, indentAdj, src, slprint.Config{Mode: printerMode, Tabwidth: tabWidth})
	if res == nil {
		return "", nil
	}

	if err != nil {
		return "", err
	}

	_, fn := filepath.Split(filename)
	ext := filepath.Ext(fn)
	fn = fn[:len(fn)-len(ext)] + ".tmp"
	outfn := filepath.Join(*outDir, fn)

	err = ioutil.WriteFile(outfn, res, 0644)
	if err != nil {
		return "", err
	}
	outFiles = append(outFiles, outfn)

	return outfn, err
}

// readFile reads the contents of filename, described by info.
// If in is non-nil, readFile reads directly from it.
// Otherwise, readFile opens and reads the file itself,
// with the number of concurrently-open files limited by fdSem.
func readFile(filename string, info fs.FileInfo) ([]byte, error) {
	fdSem <- true
	var err error
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	in := f
	defer func() {
		f.Close()
		<-fdSem
	}()

	// Compute the file's size and read its contents with minimal allocations.
	//
	// If we have the FileInfo from filepath.WalkDir, use it to make
	// a buffer of the right size and avoid ReadAll's reallocations.
	//
	// If the size is unknown (or bogus, or overflows an int), fall back to
	// a size-independent ReadAll.
	size := -1
	if info != nil && info.Mode().IsRegular() && int64(int(info.Size())) == info.Size() {
		size = int(info.Size())
	}
	if size+1 <= 0 {
		// The file is not known to be regular, so we don't have a reliable size for it.
		var err error
		src, err := io.ReadAll(in)
		if err != nil {
			return nil, err
		}
		return src, nil
	}

	// We try to read size+1 bytes so that we can detect modifications: if we
	// read more than size bytes, then the file was modified concurrently.
	// (If that happens, we could, say, append to src to finish the read, or
	// proceed with a truncated buffer — but the fact that it changed at all
	// indicates a possible race with someone editing the file, so we prefer to
	// stop to avoid corrupting it.)
	src := make([]byte, size+1)
	n, err := io.ReadFull(in, src)
	switch err {
	case nil, io.EOF, io.ErrUnexpectedEOF:
		// io.ReadFull returns io.EOF (for an empty file) or io.ErrUnexpectedEOF
		// (for a non-empty file) if the file was changed unexpectedly. Continue
		// with comparing file sizes in those cases.
	default:
		return nil, err
	}
	if n < size {
		return nil, fmt.Errorf("error: size of %s changed during reading (from %d to %d bytes)", filename, size, n)
	} else if n > size {
		return nil, fmt.Errorf("error: size of %s changed during reading (from %d to >=%d bytes)", filename, size, len(src))
	}
	return src[:n], nil
}

func main() {
	// Arbitrarily limit in-flight work to 2MiB times the number of threads.
	//
	// The actual overhead for the parse tree and output will depend on the
	// specifics of the file, but this at least keeps the footprint of the process
	// roughly proportional to GOMAXPROCS.
	// maxWeight := (2 << 20) * int64(runtime.GOMAXPROCS(0))
	// s := newSequencer(maxWeight, os.Stdout, os.Stderr)

	// call goslMain in a separate function
	// so that it can use defer and have them
	// run before the exit.
	goslMain()
	// os.Exit(s.GetExitCode())
}

func goslMain() {
	flag.Usage = usage
	flag.Parse()

	initParserMode()

	if *outDir != "" {
		os.MkdirAll(*outDir, 0755)
	}

	args := flag.Args()
	if len(args) == 0 {
		fmt.Printf("at least one file name must be passed\n")
		return
	}

	for _, arg := range args {
		switch info, err := os.Stat(arg); {
		case err != nil:
			fmt.Println(err)
		case !info.IsDir():
			// Non-directory arguments are always formatted.
			arg := arg
			processFile(arg, info)
		default:
			// Directories are walked, ignoring non-Go files.
			err := filepath.WalkDir(arg, func(path string, f fs.DirEntry, err error) error {
				if err != nil || !isGoFile(f) {
					return err
				}
				info, err := f.Info()
				if err != nil {
					return nil
				}
				_, err = processFile(path, info)
				return err
			})
			fmt.Println(err)
		}
	}

	extractFiles(outFiles)
}
