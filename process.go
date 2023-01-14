// Copyright (c) 2022, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// heavily modified from go src/cmd/gofmt/internal.go:

// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/goki/gosl/alignsl"
	"github.com/goki/gosl/slprint"
	"github.com/goki/ki/ints"
	"golang.org/x/exp/slices"
	"golang.org/x/tools/go/packages"
)

// does all the file processing
func ProcessFiles(paths []string) (map[string][]byte, error) {
	fls := FilesFromPaths(paths)
	sls := ExtractFiles(fls) // extract files to shader/*.go

	pf := "./" + *outDir
	pkgs, err := packages.Load(&packages.Config{Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedTypesSizes}, pf)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	if len(pkgs) != 1 {
		err := fmt.Errorf("More than one package for path: %v", pf)
		log.Println(err)
		return nil, err
	}
	pkg := pkgs[0]

	if len(pkg.GoFiles) == 0 {
		err := fmt.Errorf("No Go files found in package: %+v", pkg)
		log.Println(err)
		return nil, err
	}
	// fmt.Printf("go files: %+v", pkg.GoFiles)
	// return nil, err

	for fn := range sls {
		gofn := fn + ".go"
		fmt.Printf("###################################\nProcessing file: %s\n\t (ignore any 'Entry point not found' warnings for include-only files)\n", gofn)

		serr := alignsl.CheckPackage(pkg)
		if serr != nil {
			fmt.Println(serr)
		}

		var afile *ast.File
		var fpos token.Position
		for _, sy := range pkg.Syntax {
			pos := pkg.Fset.Position(sy.Package)
			_, posfn := filepath.Split(pos.Filename)
			if posfn == gofn {
				fpos = pos
				afile = sy
				break
			}
		}
		if afile == nil {
			fmt.Printf("Warning: File named: %s not found in processed package\n", gofn)
			continue
		}

		var buf bytes.Buffer
		cfg := slprint.Config{Mode: printerMode, Tabwidth: tabWidth, ExcludeFuns: excludeFunMap}
		cfg.Fprint(&buf, pkg, fpos, afile)
		// ioutil.WriteFile(filepath.Join(*outDir, fn+".tmp"), buf.Bytes(), 0644)
		slfix := SlEdits(buf.Bytes())
		exsl := ExtractHLSL(slfix)
		sls[fn] = exsl

		if !*keepTmp {
			os.Remove(fpos.Filename)
		}

		slfn := filepath.Join(*outDir, fn+".hlsl")
		ioutil.WriteFile(slfn, exsl, 0644)
		CompileFile(fn + ".hlsl")
	}
	return sls, nil
}

func ExtractFiles(files []string) map[string][]byte {
	sls := map[string][][]byte{}
	key := []byte("//gosl: ")
	start := []byte("start")
	hlsl := []byte("hlsl")
	end := []byte("end")
	nl := []byte("\n")
	include := []byte("#include")

	for _, fn := range files {
		buf, err := os.ReadFile(fn)
		if err != nil {
			continue
		}
		lines := bytes.Split(buf, nl)

		inReg := false
		inHlsl := false
		var outLns [][]byte
		slFn := ""
		for _, ln := range lines {
			isKey := bytes.HasPrefix(ln, key)
			var keyStr []byte
			if isKey {
				keyStr = ln[len(key):]
				// fmt.Printf("key: %s\n", string(keyStr))
			}
			switch {
			case inReg && isKey && bytes.HasPrefix(keyStr, end):
				if inHlsl {
					outLns = append(outLns, ln)
				}
				sls[slFn] = outLns
				inReg = false
				inHlsl = false
			case inReg:
				for pkg := range LoadedPackageNames { // remove package prefixes
					if !bytes.Contains(ln, include) {
						ln = bytes.ReplaceAll(ln, []byte(pkg+"."), []byte{})
					}
				}
				outLns = append(outLns, ln)
			case isKey && bytes.HasPrefix(keyStr, start):
				inReg = true
				slFn = string(keyStr[len(start)+1:])
				outLns = sls[slFn]
			case isKey && bytes.HasPrefix(keyStr, hlsl):
				inReg = true
				inHlsl = true
				slFn = string(keyStr[len(hlsl)+1:])
				outLns = sls[slFn]
				outLns = append(outLns, ln)
			}
		}
	}

	rsls := make(map[string][]byte)
	for fn, lns := range sls {
		outfn := filepath.Join(*outDir, fn+".go")
		olns := [][]byte{}
		olns = append(olns, []byte("package main"))
		olns = append(olns, []byte(`import "math"`))
		olns = append(olns, lns...)
		res := bytes.Join(olns, nl)
		ioutil.WriteFile(outfn, res, 0644)
		cmd := exec.Command("goimports", "-w", fn+".go") // get imports
		cmd.Dir, _ = filepath.Abs(*outDir)
		out, err := cmd.CombinedOutput()
		_ = out
		// fmt.Printf("\n################\ngoimports output for: %s\n%s\n", outfn, out)
		if err != nil {
			log.Println(err)
		}
		rsls[fn] = bytes.Join(lns, nl)
	}

	return rsls
}

func ExtractHLSL(buf []byte) []byte {
	key := []byte("//gosl: ")
	hlsl := []byte("hlsl")
	end := []byte("end")
	nl := []byte("\n")
	stComment := []byte("/*")
	edComment := []byte("*/")
	comment := []byte("// ")
	pack := []byte("package")
	imp := []byte("import")
	lparen := []byte("(")
	rparen := []byte(")")

	lines := bytes.Split(buf, nl)

	mx := ints.MinInt(10, len(lines))
	stln := 0
	gotImp := false
	for li := 0; li < mx; li++ {
		ln := lines[li]
		switch {
		case bytes.HasPrefix(ln, pack):
			stln = li + 1
		case bytes.HasPrefix(ln, imp):
			if bytes.HasSuffix(ln, lparen) {
				gotImp = true
			} else {
				stln = li + 1
			}
		case gotImp && bytes.HasPrefix(ln, rparen):
			stln = li + 1
		}
	}

	lines = lines[stln:] // get rid of package, import

	inHlsl := false
	for li := 0; li < len(lines); li++ {
		ln := lines[li]
		isKey := bytes.HasPrefix(ln, key)
		var keyStr []byte
		if isKey {
			keyStr = ln[len(key):]
			// fmt.Printf("key: %s\n", string(keyStr))
		}
		switch {
		case inHlsl && isKey && bytes.HasPrefix(keyStr, end):
			slices.Delete(lines, li, li+1)
			li--
			inHlsl = false
		case inHlsl:
			switch {
			case bytes.HasPrefix(ln, stComment) || bytes.HasPrefix(ln, edComment):
				slices.Delete(lines, li, li+1)
				li--
			case bytes.HasPrefix(ln, comment):
				lines[li] = ln[3:]
			}
		case isKey && bytes.HasPrefix(keyStr, hlsl):
			inHlsl = true
			slices.Delete(lines, li, li+1)
			li--
		}
	}
	return bytes.Join(lines, nl)
}

func CompileFile(fn string) error {
	ext := filepath.Ext(fn)
	ofn := fn[:len(fn)-len(ext)] + ".spv"
	cmd := exec.Command("glslc", "-fshader-stage=compute", "-o", ofn, fn)
	cmd.Dir, _ = filepath.Abs(*outDir)
	out, err := cmd.CombinedOutput()
	fmt.Printf("\n-----------------------------\nglslc output for: %s\n%s\n", fn, out)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}
