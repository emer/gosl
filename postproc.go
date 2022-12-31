// Copyright 2022 The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func extractFiles(files []string) {

	sls := make(map[string][][]byte)

	key := []byte("//gosl: ")
	start := []byte("start")
	hlsl := []byte("hlsl")
	end := []byte("end")
	nl := []byte("\n")
	stComment := []byte("/*")
	edComment := []byte("*/")
	comment := []byte("// ")

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
				sls[slFn] = outLns
				inReg = false
				inHlsl = false
			case inReg:
				if inHlsl {
					switch {
					case bytes.HasPrefix(ln, stComment) || bytes.HasPrefix(ln, edComment):
						// skip
					case bytes.HasPrefix(ln, comment):
						outLns = append(outLns, ln[3:])
					default:
						outLns = append(outLns, ln)
					}
				} else {
					outLns = append(outLns, ln)
				}
			case isKey && bytes.HasPrefix(keyStr, start):
				inReg = true
				slFn = string(keyStr[len(start)+1:])
				outLns = sls[slFn]
			case isKey && bytes.HasPrefix(keyStr, hlsl):
				inReg = true
				inHlsl = true
				slFn = string(keyStr[len(hlsl)+1:])
				outLns = sls[slFn]
			}
		}
		if !*keepTmp {
			os.Remove(fn)
		}
	}

	for fn, lns := range sls {
		fn += ".hlsl"
		outfn := filepath.Join(*outDir, fn)
		res := bytes.Join(lns, nl)
		ioutil.WriteFile(outfn, res, 0644)
		compileFile(fn)
	}
}

func compileFile(fn string) error {
	ext := filepath.Ext(fn)
	ofn := fn[:len(fn)-len(ext)] + ".spv"
	cmd := exec.Command("glslc", "-fshader-stage=compute", "-o", ofn, fn)
	cmd.Dir, _ = filepath.Abs(*outDir)
	if err := cmd.Run(); err != nil {
		log.Println(err)
		return err
	}
	return nil
}
