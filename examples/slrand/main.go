// Copyright (c) 2022, The Goki Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/emer/emergent/timer"
	"github.com/goki/gosl/sltype"
	"github.com/goki/ki/ints"
	"github.com/goki/vgpu/vgpu"
)

// note: standard one to use is plain "gosl" which should be go install'd

//go:generate ../../gosl -keep slrand.go

func init() {
	// must lock main thread for gpu!  this also means that vulkan must be used
	// for gogi/oswin eventually if we want gui and compute
	runtime.LockOSThread()
}

func main() {
	if vgpu.InitNoDisplay() != nil {
		return
	}

	gp := vgpu.NewComputeGPU()
	// vgpu.Debug = true
	gp.Config("slrand")

	// gp.PropsString(true) // print

	// n := 10
	n := 10000000

	dataC := make([]Rnds, n)
	dataG := make([]Rnds, n)

	cpuTmr := timer.Time{}
	cpuTmr.Start()

	seed := sltype.Uint2{0, 0}

	for i := range dataC {
		d := &dataC[i]
		d.RndGen(seed, uint32(i))
	}
	cpuTmr.Stop()

	sy := gp.NewComputeSystem("slrand")
	pl := sy.NewPipeline("slrand")
	pl.AddShaderFile("slrand", vgpu.ComputeShader, "shaders/slrand.spv")

	vars := sy.Vars()
	setc := vars.AddSet()
	setd := vars.AddSet()

	ctrv := setc.AddStruct("Counter", int(unsafe.Sizeof(seed)), 1, vgpu.Uniform, vgpu.ComputeShader)
	datav := setd.AddStruct("Data", int(unsafe.Sizeof(Rnds{})), n, vgpu.Storage, vgpu.ComputeShader)

	setc.ConfigVals(1) // one val per var
	setd.ConfigVals(1) // one val per var
	sy.Config()        // configures vars, allocates vals, configs pipelines..

	gpuFullTmr := timer.Time{}
	gpuFullTmr.Start()

	// this copy is pretty fast -- most of time is below
	cvl, _ := ctrv.Vals.ValByIdxTry(0)
	cvl.CopyFromBytes(unsafe.Pointer(&seed))
	dvl, _ := datav.Vals.ValByIdxTry(0)
	dvl.CopyFromBytes(unsafe.Pointer(&dataG[0]))

	// gpuFullTmr := timer.Time{}
	// gpuFullTmr.Start()

	sy.Mem.SyncToGPU()

	vars.BindDynValIdx(0, "Counter", 0)
	vars.BindDynValIdx(1, "Data", 0)

	sy.CmdResetBindVars(sy.CmdPool.Buff, 0)

	// gpuFullTmr := timer.Time{}
	// gpuFullTmr.Start()

	gpuTmr := timer.Time{}
	gpuTmr.Start()

	pl.ComputeCommand(n, 1, 1)
	sy.ComputeSubmitWait()

	gpuTmr.Stop()

	sy.Mem.SyncValIdxFmGPU(1, "Data", 0) // this is about same as SyncToGPU
	dvl.CopyToBytes(unsafe.Pointer(&dataG[0]))

	gpuFullTmr.Stop()

	anyDiffEx := false
	anyDiffTol := false
	mx := ints.MinInt(n, 5)
	fmt.Printf("Idx\tDif(Ex,Tol)\t   CPU   \t  then GPU\n")
	for i := 0; i < n; i++ {
		dc := &dataC[i]
		dg := &dataG[i]
		smEx, smTol := dc.IsSame(dg)
		if !smEx {
			anyDiffEx = true
		}
		if !smTol {
			anyDiffTol = true
		}
		if i > mx {
			continue
		}
		exS := " "
		if !smEx {
			exS = "*"
		}
		tolS := " "
		if !smTol {
			tolS = "*"
		}
		fmt.Printf("%d\t%s %s\t%s\n\t\t%s\n", i, exS, tolS, dc.String(), dg.String())
	}
	fmt.Printf("\n")

	if anyDiffEx {
		fmt.Printf("ERROR: Differences between CPU and GPU detected at Exact level (excludes Gauss)\n\n")
	}
	if anyDiffTol {
		fmt.Printf("ERROR: Differences between CPU and GPU detected at Tolerance level of %g\n\n", Tol)
	}

	cpu := cpuTmr.TotalSecs()
	gpu := gpuTmr.TotalSecs()
	fmt.Printf("N: %d\t CPU: %6.4g\t GPU: %6.4g\t Full: %6.4g\t CPU/GPU: %6.4g\n", n, cpu, gpu, gpuFullTmr.TotalSecs(), cpu/gpu)

	sy.Destroy()
	gp.Destroy()
	vgpu.Terminate()
}
