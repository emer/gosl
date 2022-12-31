// Copyright (c) 2022, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "math"

//gosl: start basic

// DataStruct has the test data
type DataStruct struct {
	Raw   float32 `desc:"raw value"`
	Integ float32 `desc:"integrated value"`
	Exp   float32 `desc:"exp of integ"`
	Pad2  float32 `desc:"must pad to 4 floats"`
}

// ParamStruct has the test params
type ParamStruct struct {
	Tau float32 `desc:"rate constant in msec"`
	Dt  float32 `desc:"1/Tau"`
}

// IntegFmRaw computes integrated value from current raw value
func (ps *ParamStruct) IntegFmRaw(ds *DataStruct) {
	ds.Integ += ps.Dt * (ds.Raw - ds.Integ)
	ds.Exp = float32(math.Exp(-float64(ds.Integ)))
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

//gosl: main basic
// [[vk::binding(0, 0)]] uniform ParamStruct Params;
// [[vk::binding(0, 1)]] RWStructuredBuffer<DataStruct> In;
// [numthreads(1, 1, 1)]
// void main(uint3 idx : SV_DispatchThreadID)
// {
//     Params.IntegFmRaw(&In[idx.x]);
// }
//gosl: end basic
