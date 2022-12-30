// Copyright 2022 The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

func extractFiles(files []string) {

	sls := make(map[string][][]byte)

	dir := []byte("//gosl: ")
	start := []byte("start")
	main := []byte("main")
	end := []byte("end")
	nl := []byte("\n")

	for _, fn := range files {
		buf, err := os.ReadFile(fn)
		if err != nil {
			continue
		}
		lines := bytes.Split(buf, nl)

		inReg := false
		inMain := false
		var outLns [][]byte
		slFn := ""
		for _, ln := range lines {
			isDir := bytes.HasPrefix(ln, dir)
			var dirStr []byte
			if isDir {
				dirStr = ln[len(dir):]
				fmt.Printf("dir: %s\n", string(dirStr))
			}
			switch {
			case inReg && isDir && bytes.HasPrefix(dirStr, end):
				sls[slFn] = outLns
				inReg = false
				inMain = false
			case inReg:
				if inMain {
					if len(ln) > 3 {
						outLns = append(outLns, ln[3:])
					} else {
						outLns = append(outLns, ln)
					}
				} else {
					outLns = append(outLns, ln)
				}
			case isDir && bytes.HasPrefix(dirStr, start):
				inReg = true
				slFn = string(dirStr[len(start)+1:])
				outLns = sls[slFn]
			case isDir && bytes.HasPrefix(dirStr, main):
				inReg = true
				inMain = true
				slFn = string(dirStr[len(main)+1:])
				outLns = sls[slFn]
			}
		}
	}

	for fn, lns := range sls {
		fn += ".hlsl"
		outfn := filepath.Join(*outDir, fn)
		res := bytes.Join(lns, nl)
		ioutil.WriteFile(outfn, res, 0644)
	}
}
