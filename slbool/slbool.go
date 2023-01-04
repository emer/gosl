// Copyright (c) 2019, The Emergent Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
package slbool defines a HLSL int32 Bool type -- the standard bool type doesn't
work for unclear reasons, but also padding and alignment issues are problematic.
*/
package slbool

type Bool int32

const (
	False Bool = 0
	True  Bool = 1
)

func IsTrue(b Bool) bool {
	return b == True
}

func IsFalse(b Bool) bool {
	return b == False
}

func FromBool(b bool) Bool {
	if b {
		return True
	}
	return False
}
