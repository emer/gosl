// Copyright (c) 2022, The Goki Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package alignsl

import (
	"fmt"
	"go/types"

	"golang.org/x/tools/go/packages"
)

var Sizes types.Sizes

func CheckStruct(st *types.Struct) {
	var flds []*types.Var
	nf := st.NumFields()
	if nf == 0 {
		return
	}
	for i := 0; i < nf; i++ {
		fl := st.Field(i)
		flds = append(flds, fl)
		ft := fl.Type()
		ut := ft.Underlying()
		if bt, isBasic := ut.(*types.Basic); isBasic {
			kind := bt.Kind()
			if !(kind == types.Uint32 || kind == types.Int32 || kind == types.Float32) {
				fmt.Printf("    %s:  basic type != [U]Int32 or Float32: %s\n", fl.Name(), bt.String())
			}
		} else {
			if _, is := ut.(*types.Struct); is {

			} else {
				fmt.Printf("    %s:  unsupported type: %s\n", fl.Name(), ft.String())
			}
		}
	}
	offs := Sizes.Offsetsof(flds)
	last := Sizes.Sizeof(flds[nf-1].Type())
	totsz := int(offs[nf-1] + last)
	if totsz%16 != 0 {
		fmt.Printf("    total size: %d not even multiple of 16\n", totsz)
	}
}

func CheckPackage(pkg *packages.Package) {
	fmt.Printf("\nstruct type alignment checking\n")
	fmt.Printf("    checks that struct sizes are an even multiple of 16 bytes (4 float32's)\n")
	fmt.Printf("    and are of 32 bit types: [U]Int32, Float32\n")
	// fmt.Printf("package: %s\n", pkg.Name)
	Sizes = pkg.TypesSizes
	sc := pkg.Types.Scope()
	CheckScope(sc, 0)
}

func CheckScope(sc *types.Scope, level int) {
	nms := sc.Names()
	ntyp := 0
	for _, nm := range nms {
		ob := sc.Lookup(nm)
		tp := ob.Type()
		if tp == nil {
			continue
		}
		if nt, is := tp.(*types.Named); is {
			ut := nt.Underlying()
			if ut == nil {
				continue
			}
			if st, is := ut.(*types.Struct); is {
				fmt.Printf("%s\n", nt.Obj().Name())
				CheckStruct(st)
				ntyp++
			}
		}
	}
	if ntyp == 0 {
		for i := 0; i < sc.NumChildren(); i++ {
			cs := sc.Child(i)
			CheckScope(cs, level+1)
		}
	}
}
