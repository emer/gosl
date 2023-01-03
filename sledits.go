// Copyright 2022 The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"strings"
)

// moveLines moves the st,ed region to 'to' line
func moveLines(lines *[][]byte, to, st, ed int) {
	mvln := (*lines)[st:ed]
	btwn := (*lines)[to:st]
	aft := (*lines)[ed:len(*lines)]
	nln := make([][]byte, to, len(*lines))
	copy(nln, (*lines)[:to])
	nln = append(nln, mvln...)
	nln = append(nln, btwn...)
	nln = append(nln, aft...)
	*lines = nln
}

// slEdits performs post-generation edits for hlsl
// * moves hlsl segments around, e.g., methods
// into their proper classes
// * fixes printf, slice other common code
func slEdits(src []byte) []byte {
	// return src // uncomment to show original without edits
	nl := []byte("\n")
	lines := bytes.Split(src, nl)

	lines = slEditsMethMove(lines)
	slEditsReplace(lines)

	return bytes.Join(lines, nl)
}

// slEditsMethMove moves hlsl segments around, e.g., methods
// into their proper classes
func slEditsMethMove(lines [][]byte) [][]byte {
	type sted struct {
		st, ed int
	}
	classes := map[string]sted{}

	class := []byte("struct ")
	slmark := []byte("<<<<")
	slend := []byte(">>>>")

	endclass := "EndClass: "
	method := "Method: "
	endmethod := "EndMethod"

	lastMethSt := -1
	var lastMeth string
	curComSt := -1
	lastComSt := -1
	lastComEd := -1

	li := 0
	for {
		if li >= len(lines) {
			break
		}
		ln := lines[li]
		if len(ln) >= 2 && string(ln[0:1]) == "//" {
			if curComSt >= 0 {
				lastComEd = li
			} else {
				curComSt = li
				lastComSt = li
				lastComEd = li
			}
		} else {
			curComSt = -1
		}

		switch {
		case bytes.HasPrefix(ln, class):
			cl := string(ln[len(class):])
			if idx := strings.Index(cl, "("); idx > 0 {
				cl = cl[:idx]
			} else if idx := strings.Index(cl, "{"); idx > 0 { // should have
				cl = cl[:idx]
			}
			cl = strings.TrimSpace(cl)
			classes[cl] = sted{st: li}
			// fmt.Printf("cl: %s at %d\n", cl, li)
		case bytes.HasPrefix(ln, slmark) && bytes.HasSuffix(ln, slend):
			tag := string(ln[4 : len(ln)-4])
			// fmt.Printf("tag: %s at: %d\n", tag, li)
			switch {
			case strings.HasPrefix(tag, endclass):
				cl := tag[len(endclass):]
				st := classes[cl]
				classes[cl] = sted{st: st.st, ed: li - 1}
				lines = append(lines[:li], lines[li+1:]...) // delete marker
				// fmt.Printf("cl: %s at %v\n", cl, classes[cl])
				li--
			case strings.HasPrefix(tag, method):
				cl := tag[len(method):]
				lines = append(lines[:li], lines[li+1:]...) // delete marker
				li--
				lastMeth = cl
				if lastComEd == li {
					lines = append(lines[:lastComSt], lines[lastComEd+1:]...) // delete comments
					lastMethSt = lastComSt
					li = lastComSt - 1
				} else {
					lastMethSt = li + 1
				}
			case tag == endmethod:
				se, ok := classes[lastMeth]
				if ok {
					lines = append(lines[:li], lines[li+1:]...) // delete marker
					moveLines(&lines, se.ed, lastMethSt, li+1)  // extra blank
					classes[lastMeth] = sted{st: se.st, ed: se.ed + ((li + 1) - lastMethSt)}
					li -= 2
				}
			}
		}
		li++
	}
	return lines
}

type Replace struct {
	From, To []byte
}

var Replaces = []Replace{
	{[]byte("float32"), []byte("float")},
	{[]byte("float64"), []byte("double")},
	{[]byte("uint32"), []byte("uint")},
	{[]byte("int32"), []byte("int")},
	{[]byte("math.Exp("), []byte("exp(")},
	{[]byte("mat32.Exp("), []byte("exp(")},
	{[]byte("mat32.Log("), []byte("log(")},
	{[]byte("mat32.Pow("), []byte("pow(")},
	{[]byte("mat32.Cos("), []byte("cos(")},
	{[]byte("mat32.Sin("), []byte("sin(")},
	{[]byte("mat32.Abs("), []byte("abs(")},
	{[]byte("mat32.FastExp("), []byte("FastExp(")},
	{[]byte("math.Float32frombits("), []byte("asfloat(")},
	// {[]byte(""), []byte("")},
	// {[]byte(""), []byte("")},
	// {[]byte(""), []byte("")},
}

// slEditsReplace replaces Go with equivalent HLSL code
func slEditsReplace(lines [][]byte) {
	for li, ln := range lines {
		for _, r := range Replaces {
			ln = bytes.Replace(ln, r.From, r.To, -1)
		}
		lines[li] = ln
	}
}
