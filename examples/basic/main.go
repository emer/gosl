// Copyright (c) 2022, The Goki Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math/rand"
	"runtime"
	"unsafe"

	"github.com/goki/vgpu/vgpu"
)

func init() {
	// must lock main thread for gpu!  this also means that vulkan must be used
	// for gogi/oswin eventually if we want gui and compute
	runtime.LockOSThread()
}

var TheGPU *vgpu.GPU

func main() {
	if vgpu.Init() != nil {
		return
	}

	gp := vgpu.NewComputeGPU()
	gp.Debug = true
	gp.Config("basic")
	TheGPU = gp

	// gp.PropsString(true) // print

	n := 32

	pars := &ParamStruct{}
	pars.Defaults()

	data := make([]DataStruct, n)
	for i := range data {
		d := &data[i]
		d.Raw = rand.Float32()
		d.Integ = 0
	}

	sy := gp.NewComputeSystem("basic")
	pl := sy.NewPipeline("basic")
	pl.AddShaderFile("basic", vgpu.ComputeShader, "shaders/basic.spv")

	vars := sy.Vars()
	setp := vars.AddSet()
	setd := vars.AddSet()

	parsv := setp.AddStruct("Params", int(unsafe.Sizeof(ParamStruct{})), 1, vgpu.Uniform, vgpu.ComputeShader)
	datav := setd.AddStruct("Data", int(unsafe.Sizeof(DataStruct{})), n, vgpu.Storage, vgpu.ComputeShader)

	setp.ConfigVals(1) // one val per var
	setd.ConfigVals(1) // one val per var
	sy.Config()        // configures vars, allocates vals, configs pipelines..

	pvl, _ := parsv.Vals.ValByIdxTry(0)
	pvl.CopyFromBytes(unsafe.Pointer(pars))
	dvl, _ := datav.Vals.ValByIdxTry(0)
	dvl.CopyFromBytes(unsafe.Pointer(&data[0]))

	sy.Mem.SyncToGPU()

	vars.BindDynValIdx(0, "Params", 0)
	vars.BindDynValIdx(1, "Data", 0)

	sy.CmdResetBindVars(sy.CmdPool.Buff, 0)
	pl.RunComputeWait(sy.CmdPool.Buff, n, 1, 1)
	// note: could use semaphore here instead of waiting on the compute

	sy.Mem.SyncValIdxFmGPU(1, "Data", 0)
	dvl.CopyToBytes(unsafe.Pointer(&data[0]))

	for i := range data {
		d := &data[i]
		fmt.Printf("%d\tRaw: %g\tInteg: %g\n", i, d.Raw, d.Integ)
	}
	fmt.Printf("\n")

	sy.Destroy()
	gp.Destroy()
	vgpu.Terminate()
}
