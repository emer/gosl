// Copyright (c) 2019, The Emergent Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
package slbool defines a HLSL friendly int32 Bool type.
The standard HLSL bool type causes obscure errors,
and the int32 obeys the 4 byte basic alignment requirements.

gosl automatically converts this Go code into appropriate HLSL code.
*/
package slbool

type Bool int32

const (
	False Bool = 0
	True  Bool = 1
)

func (b *Bool) IsTrue() bool {
	return *b == True
}

func (b *Bool) IsFalse() bool {
	return *b == False
}

func (b *Bool) SetBool(bb bool) {
	*b = FromBool(bb)
}

func (b *Bool) String() string {
	if b.IsTrue() {
		return "true"
	}
	return "false"
}

func (b *Bool) FromString(s string) {
	if s == "true" || s == "True" {
		b.SetBool(true)
	} else {
		b.SetBool(false)
	}

}

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
