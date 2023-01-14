// Copyright (c) 2022, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// These are Go versions of the same Philox4x32 random number generator
// functions available in .HLSL.

// The Philox4x32 algorithm is also used on CUDA and in Tensorflow.
// It is a counter based RNG where the random number is
// a direct function of an input state.

// https://en.wikipedia.org/wiki/Counter-based_random_number_generator_(CBRNG)
// https://github.com/DEShawResearch/random123
// https://github.com/DEShawResearch/random123/blob/main/include/Random123/philox.h

// The Go code is very similar to the HLSL code, except that HLSL does not (yet)
// support uint64 types, so it must use a more complex version of the
// core MulHiLo32 function.


