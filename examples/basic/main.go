// Copyright (c) 2022, The Goki Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math/rand"
	"runtime"
	"unsafe"

	"github.com/emer/emergent/timer"
	"github.com/goki/ki/ints"
	"github.com/goki/vgpu/vgpu"
)

// note: standard one to use is plain "gosl" which should be go install'd

//go:generate ../../gosl compute.go

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
	// gp.Debug = true
	gp.Config("basic")
	TheGPU = gp

	// gp.PropsString(true) // print

	n := 10000000

	pars := &ParamStruct{}
	pars.Defaults()

	data := make([]DataStruct, n)
	for i := range data {
		d := &data[i]
		d.Raw = rand.Float32()
		d.Integ = 0
	}

	cpuTmr := timer.Time{}
	cpuTmr.Start()
	for i := range data {
		d := &data[i]
		pars.IntegFmRaw(d)
	}
	cpuTmr.Stop()

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

	gpuFullTmr := timer.Time{}
	gpuFullTmr.Start()

	// this copy is pretty fast -- most of time is below
	pvl, _ := parsv.Vals.ValByIdxTry(0)
	pvl.CopyFromBytes(unsafe.Pointer(pars))
	dvl, _ := datav.Vals.ValByIdxTry(0)
	dvl.CopyFromBytes(unsafe.Pointer(&data[0]))

	// gpuFullTmr := timer.Time{}
	// gpuFullTmr.Start()

	sy.Mem.SyncToGPU()

	vars.BindDynValIdx(0, "Params", 0)
	vars.BindDynValIdx(1, "Data", 0)

	sy.CmdResetBindVars(sy.CmdPool.Buff, 0)

	// gpuFullTmr := timer.Time{}
	// gpuFullTmr.Start()

	gpuTmr := timer.Time{}
	gpuTmr.Start()

	pl.RunComputeWait(sy.CmdPool.Buff, n, 1, 1)
	// note: could use semaphore here instead of waiting on the compute

	gpuTmr.Stop()

	sy.Mem.SyncValIdxFmGPU(1, "Data", 0) // this is about same as SyncToGPU
	dvl.CopyToBytes(unsafe.Pointer(&data[0]))

	gpuFullTmr.Stop()

	mx := ints.MinInt(n, 5)
	for i := 0; i < mx; i++ {
		d := &data[i]
		fmt.Printf("%d\tRaw: %g\tInteg: %g\tExp: %g\n", i, d.Raw, d.Integ, d.Exp)
	}
	fmt.Printf("\n")

	fmt.Printf("N: %d\t CPU: %6.4g\t GPU: %6.4g\t Full: %6.4g\n", n, cpuTmr.TotalSecs(), gpuTmr.TotalSecs(), gpuFullTmr.TotalSecs())

	sy.Destroy()
	gp.Destroy()
	vgpu.Terminate()
}
