// Copyright (c) 2022, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slrand

import "github.com/goki/mat32"

// These are Go versions of the same Philox2x32 based random number generator
// functions available in .HLSL.

// Uint2 is the Go version of the HLSL uint2
type Uint2 struct {
	X, Y uint32
}

// Float2 is the Go version of the HLSL float2
type Float2 struct {
	X, Y float32
}

// MulHiLo64 is the fast, simpler version when 64 bit uints become available
func MulHiLo64(a, b uint32) (lo, hi uint32) {
	prod := uint64(a) * uint64(b)
	hi = uint32(prod >> 32)
	lo = uint32(prod)
	return
}

// Philox2x32round does one round of updating of the counter
func Philox2x32round(counter *Uint2, key uint32) {
	lo, hi := MulHiLo64(0xD256D193, counter.X)
	counter.X = hi ^ key ^ counter.Y
	counter.Y = lo
}

// Philox2x32bumpkey does one round of updating of the key
func Philox2x32bumpkey(key *uint32) {
	*key += 0x9E3779B9
}

// Philox2x32 implements the stateless counter-based RNG algorithm
// returning a random number as 2 uint3232 32 bit values, given a
// counter and key input that determine the result.
func Philox2x32(counter Uint2, key uint32) Uint2 {
	Philox2x32round(&counter, key) // 1
	Philox2x32bumpkey(&key)
	Philox2x32round(&counter, key) // 2
	Philox2x32bumpkey(&key)
	Philox2x32round(&counter, key) // 3
	Philox2x32bumpkey(&key)
	Philox2x32round(&counter, key) // 4
	Philox2x32bumpkey(&key)
	Philox2x32round(&counter, key) // 5
	Philox2x32bumpkey(&key)
	Philox2x32round(&counter, key) // 6
	Philox2x32bumpkey(&key)
	Philox2x32round(&counter, key) // 7
	Philox2x32bumpkey(&key)
	Philox2x32round(&counter, key) // 8
	Philox2x32bumpkey(&key)
	Philox2x32round(&counter, key) // 9
	Philox2x32bumpkey(&key)

	Philox2x32round(&counter, key) // 10
	return counter
}

// Uint32ToFloat converts a uint32 32 bit integer into a 32 bit float
// in the [0..1) interval (i.e., exclusive of 1).
func Uint32ToFloat(val uint32) float32 {
	const factor = float32(1.) / (float32(0xffffffff) + float32(1.))
	const halffactor = float32(0.5) * factor
	return float32(val)*factor + halffactor
}

// Uint32ToFloat11 converts a uint32 32 bit integer into a 32 bit float
// in the [1..1] interval.
func Uint32ToFloat11(val uint32) float32 {
	const factor = float32(1.) / (float32(0xffffffff) + float32(1.))
	const halffactor = float32(0.5) * factor
	return 2.0 * (float32(int32(val))*factor + halffactor)
}

// Uint2ToFloat01 converts two uint32 32 bit integers (Uint2)
// into two corresponding 32 bit float values (float2)
// in the [0..1) interval (i.e., exclusive of 1).
func Uint2ToFloat(val Uint2) Float2 {
	var r Float2
	r.X = Uint32ToFloat(val.X)
	r.Y = Uint32ToFloat(val.Y)
	return r
}

// CounterIncr increments the given counter as if it was
// a uint3264 integer.
func CounterIncr(counter *Uint2) {
	if counter.X == 0xffffffff {
		counter.Y++
		counter.X = 0
	} else {
		counter.X++
	}
}

////////////////////////////////////////////////////////////
//   Methods below provide a standard interface
//   with more readable names, mapping onto the Go rand methods.
//   These are what should be called by end-user code.

// RandUint2 returns two uniformly-distributed 32 unsigned integers,
// based on given counter and key.
// The counter should be incremented by 1 by calling CountIncr
// after this call as completed on all elements,
// ensuring that the next call will produce the next random number
// in the sequence.  The key should be the
// unique index of the element being updated.
func RandUint2(counter Uint2, key uint32) Uint2 {
	res := Philox2x32(counter, key)
	return res
}

// RandUint32 returns a uniformly-distributed 32 unsigned integer,
// based on given counter and key.
// The counter should be incremented by 1 by calling CountIncr
// after this call as completed on all elements,
// ensuring that the next call will produce
// the next random number in the sequence.  The key should be the
// unique index of the element being updated.
func RandUint32(counter Uint2, key uint32) uint32 {
	res := Philox2x32(counter, key)
	return res.X
}

// RandFloat2 returns two uniformly-distributed 32 floats
// in range [0..1) based on given counter and key.
// The counter should be incremented by 1 by calling CountIncr
// after this call as completed on all elements,
// ensuring that the next call will produce
// the next random number in the sequence.  The key should be the
// unique index of the element being updated.
func RandFloat2(counter Uint2, key uint32) Float2 {
	return Uint2ToFloat(RandUint2(counter, key))
}

// RandFloat returns a uniformly-distributed 32 float
// in range [0..1) based on given counter and key.
// The counter should be incremented by 1 by calling CountIncr
// after this call as completed on all elements,
// ensuring that the next call will produce
// the next random number in the sequence.  The key should be the
// unique index of the element being updated.
func RandFloat(counter Uint2, key uint32) float32 {
	return Uint32ToFloat(RandUint32(counter, key))
}

// RandFloat11 returns a uniformly-distributed 32 float
// in range [-1..1) based on given counter and key.
// The counter should be incremented by 1 by calling CountIncr
// after this call as completed on all elements,
// ensuring that the next call will produce
// the next random number in the sequence.  The key should be the
// unique index of the element being updated.
func RandFloat11(counter Uint2, key uint32) float32 {
	return Uint32ToFloat11(RandUint32(counter, key))
}

// RandBoolP returns a bool true value with probability p
func RandBoolP(counter Uint2, key uint32, p float32) bool {
	return (RandFloat(counter, key) < p)
}

func sincospi(x float32) (s, c float32) {
	const PIf = 3.1415926535897932
	s, c = mat32.Sincos(PIf * x)
	return
}

// RandNormFloat2 returns two random 32 bit floating numbers
// distributed according to the normal, Gaussian distribution
// with zero mean and unit variance.
// This is done very efficiently using the Box-Muller algorithm
// that consumes two random 32 bit uint32 values.
func RandNormFloat2(counter Uint2, key uint32) Float2 {
	ur := RandUint2(counter, key)
	var f Float2
	f.X, f.Y = sincospi(Uint32ToFloat11(ur.X))
	r := mat32.Sqrt(-2. * mat32.Log(Uint32ToFloat(ur.Y))) // u01 is guaranteed to afunc 0. hrmm.
	f.X *= r
	f.Y *= r
	return f
}

// RandNormFloat returns a random 32 bit floating number
// distributed according to the normal, Gaussian distribution
// with zero mean and unit variance.
func RandNormFloat(counter Uint2, key uint32) float32 {
	f := RandNormFloat2(counter, key)
	return f.X
}
