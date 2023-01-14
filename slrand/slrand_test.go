// Copyright (c) 2022, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slrand

import (
	"fmt"
	"testing"
)

func TestRand(t *testing.T) {
	var counter Uint2
	for i := 0; i < 100; i++ {
		fmt.Printf("%g\t%g\t%g\n", RandFloat(counter, 0), RandFloat11(counter, 1), RandNormFloat(counter, 2))
		CounterIncr(&counter)
	}
}
