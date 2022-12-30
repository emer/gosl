// Copyright 2020 The Go-Python Authors. All rights reserved.
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

// slEdits performs post-generation edits for python
// * moves python segments around, e.g., methods
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

// slEditsMethMove moves slthon segments around, e.g., methods
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
		// case bytes.Equal(ln, []byte("	:")) || bytes.Equal(ln, []byte(":")):
		// 	lines = append(lines[:li], lines[li+1:]...) // delete marker
		// 	li--
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

// slEditsReplace replaces Go with equivalent HLSL code
func slEditsReplace(lines [][]byte) {
	fmtPrintf := []byte("fmt.Printf")
	fmtSprintf := []byte("fmt.Sprintf(")
	prints := []byte("print")
	eqappend := []byte("= append(")
	elseif := []byte("else if")
	elif := []byte("elif")
	forblank := []byte("for _, ")
	fornoblank := []byte("for ")
	itoa := []byte("strconv.Itoa")
	float64p := []byte("float64(")
	float32p := []byte("float32(")
	floatp := []byte("float(")
	stringp := []byte("string(")
	strp := []byte("str(")
	stringsdot := []byte("strings.")
	copyp := []byte("copy(")
	eqgonil := []byte(" == go.nil")
	eqgonil0 := []byte(" == 0")
	negonil := []byte(" != go.nil")
	negonil0 := []byte(" != 0")

	for li, ln := range lines {
		ln = bytes.Replace(ln, float64p, floatp, -1)
		ln = bytes.Replace(ln, float32p, floatp, -1)
		ln = bytes.Replace(ln, stringp, strp, -1)
		ln = bytes.Replace(ln, forblank, fornoblank, -1)
		ln = bytes.Replace(ln, eqgonil, eqgonil0, -1)
		ln = bytes.Replace(ln, negonil, negonil0, -1)

		if bytes.Contains(ln, fmtSprintf) {
			if bytes.Contains(ln, []byte("%")) {
				ln = bytes.Replace(ln, []byte(`", `), []byte(`" % (`), -1)
			}
			ln = bytes.Replace(ln, fmtSprintf, []byte{}, -1)
		}

		if bytes.Contains(ln, fmtPrintf) {
			if bytes.Contains(ln, []byte("%")) {
				ln = bytes.Replace(ln, []byte(`", `), []byte(`" % `), -1)
			}
			ln = bytes.Replace(ln, fmtPrintf, prints, -1)
		}

		if bytes.Contains(ln, eqappend) {
			idx := bytes.Index(ln, eqappend)
			comi := bytes.Index(ln[idx+len(eqappend):], []byte(","))
			nln := make([]byte, idx-1)
			copy(nln, ln[:idx-1])
			nln = append(nln, []byte(".append(")...)
			nln = append(nln, ln[idx+len(eqappend)+comi+1:]...)
			ln = nln
		}

		for {
			if bytes.Contains(ln, stringsdot) {
				idx := bytes.Index(ln, stringsdot)
				pi := idx + len(stringsdot) + bytes.Index(ln[idx+len(stringsdot):], []byte("("))
				comi := bytes.Index(ln[pi:], []byte(","))
				nln := make([]byte, idx)
				copy(nln, ln[:idx])
				if comi < 0 {
					comi = bytes.Index(ln[pi:], []byte(")"))
					nln = append(nln, ln[pi+1:pi+comi]...)
					nln = append(nln, '.')
					meth := bytes.ToLower(ln[idx+len(stringsdot) : pi+1])
					if bytes.Equal(meth, []byte("fields(")) {
						meth = []byte("split(")
					}
					nln = append(nln, meth...)
					nln = append(nln, ln[pi+comi:]...)
				} else {
					nln = append(nln, ln[pi+1:pi+comi]...)
					nln = append(nln, '.')
					meth := bytes.ToLower(ln[idx+len(stringsdot) : pi+1])
					nln = append(nln, meth...)
					nln = append(nln, ln[pi+comi+1:]...)
				}
				ln = nln
			} else {
				break
			}
		}

		if bytes.Contains(ln, copyp) {
			idx := bytes.Index(ln, copyp)
			pi := idx + len(copyp) + bytes.Index(ln[idx+len(stringsdot):], []byte("("))
			comi := bytes.Index(ln[pi:], []byte(","))
			nln := make([]byte, idx)
			copy(nln, ln[:idx])
			nln = append(nln, ln[pi+1:pi+comi]...)
			nln = append(nln, '.')
			nln = append(nln, copyp...)
			nln = append(nln, ln[pi+comi+1:]...)
			ln = nln
		}

		if bytes.Contains(ln, itoa) {
			ln = bytes.Replace(ln, itoa, []byte(`str`), -1)
		}

		if bytes.Contains(ln, elseif) {
			ln = bytes.Replace(ln, elseif, elif, -1)
		}

		ln = bytes.Replace(ln, []byte("\t"), []byte("    "), -1)

		lines[li] = ln
	}
}
