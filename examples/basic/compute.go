// Copyright (c) 2022, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "github.com/goki/mat32"

//gosl: hlsl basic
// #include "fastexp.hlsl"
//gosl: end basic

//gosl: start basic

// DataStruct has the test data
type DataStruct struct {

	// raw value
	Raw float32 `desc:"raw value"`

	// integrated value
	Integ float32 `desc:"integrated value"`

	// exp of integ
	Exp float32 `desc:"exp of integ"`

	// must pad to multiple of 4 floats for arrays
	Pad2 float32 `desc:"must pad to multiple of 4 floats for arrays"`
}

// ParamStruct has the test params
type ParamStruct struct {

	// rate constant in msec
	Tau float32 `desc:"rate constant in msec"`

	// 1/Tau
	Dt float32 `desc:"1/Tau"`

	pad, pad1 float32
}

// IntegFmRaw computes integrated value from current raw value
func (ps *ParamStruct) IntegFmRaw(ds *DataStruct) {
	ds.Integ += ps.Dt * (ds.Raw - ds.Integ)
	ds.Exp = mat32.FastExp(-ds.Integ)
}

//gosl: end basic

// note: only core compute code needs to be in shader -- all init is done CPU-side

func (ps *ParamStruct) Defaults() {
	ps.Tau = 5
	ps.Update()
}

func (ps *ParamStruct) Update() {
	ps.Dt = 1.0 / ps.Tau
}

//gosl: hlsl basic
/*
// // note: double-commented lines required here -- binding is var, set
[[vk::binding(0, 0)]] uniform ParamStruct Params;
[[vk::binding(0, 1)]] RWStructuredBuffer<DataStruct> Data;

[numthreads(64, 1, 1)]

void main(uint3 idx : SV_DispatchThreadID) {
    Params.IntegFmRaw(Data[idx.x]);
}
*/
//gosl: end basic
