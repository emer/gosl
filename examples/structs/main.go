// Copyright (c) 2022, The Goki Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/goki/ki/ints"
	"github.com/goki/vgpu/vgpu"
)

// note: standard one to use is plain "gosl" which should be go install'd

//go:generate ../../gosl structs.go

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
	gp.Config("basic")

	// gp.PropsString(true) // print

	n := 16

	var ui1 uint32 = 1234567891
	var ui2 uint32 = 567891123
	var ui3 uint32 = 891123567

	fmt.Printf("%d  %d\n", ui1, ui2)

	var (
		UiS1  UintS1
		UiS2  UintS2
		UiS3  UintS3
		UiS4  UintS4
		UiSC1 UintSC1
		UiSC2 UintSC2
		UiSC3 UintSC3
		UiSC4 UintSC4
	)

	UiS1.Field1 = ui1
	UiS2.Field1 = ui2
	UiS2.Field2 = ui3
	UiS3.Field1 = ui1
	UiS3.Field2 = ui2
	UiS3.Field3 = ui3
	UiS4.Field1 = ui1
	UiS4.Field2 = ui2
	UiS4.Field3 = ui3
	UiS4.Field4 = ui1
	UiSC1.Fla.Field1 = ui2
	UiSC1.Flb.Field1 = ui3
	UiSC2.Fla.Field1 = ui1
	UiSC2.Fla.Field2 = ui2
	UiSC2.Flb.Field1 = ui3
	UiSC3.Fla.Field1 = ui1
	UiSC3.Fla.Field2 = ui2
	UiSC3.Fla.Field3 = ui3
	UiSC3.Flb.Field1 = ui1
	UiSC4.Fla.Field1 = ui2
	UiSC4.Fla.Field2 = ui3
	UiSC4.Fla.Field3 = ui1
	UiSC4.Fla.Field4 = ui2
	UiSC4.Flb.Field1 = ui3

	res := make([]UintS4, n)

	sy := gp.NewComputeSystem("structs")
	pl := sy.NewPipeline("structs")
	pl.AddShaderFile("structs", vgpu.ComputeShader, "shaders/structs.spv")

	vars := sy.Vars()
	setu := vars.AddSet()
	setr := vars.AddSet()

	setu.AddStruct("UiS1", int(unsafe.Sizeof(UintS1{})), 1, vgpu.Uniform, vgpu.ComputeShader)
	setu.AddStruct("UiS2", int(unsafe.Sizeof(UintS2{})), 1, vgpu.Uniform, vgpu.ComputeShader)
	setu.AddStruct("UiS3", int(unsafe.Sizeof(UintS3{})), 1, vgpu.Uniform, vgpu.ComputeShader)
	setu.AddStruct("UiS4", int(unsafe.Sizeof(UintS4{})), 1, vgpu.Uniform, vgpu.ComputeShader)
	setu.AddStruct("UiSC1", int(unsafe.Sizeof(UintSC1{})), 1, vgpu.Uniform, vgpu.ComputeShader)
	setu.AddStruct("UiSC2", int(unsafe.Sizeof(UintSC2{})), 1, vgpu.Uniform, vgpu.ComputeShader)
	setu.AddStruct("UiSC3", int(unsafe.Sizeof(UintSC3{})), 1, vgpu.Uniform, vgpu.ComputeShader)
	setu.AddStruct("UiSC4", int(unsafe.Sizeof(UintSC4{})), 1, vgpu.Uniform, vgpu.ComputeShader)

	resv := setr.AddStruct("Res", int(unsafe.Sizeof(UintS4{})), n, vgpu.Storage, vgpu.ComputeShader)

	setu.ConfigVals(1) // one val per var
	setr.ConfigVals(1) // one val per var
	sy.Config()        // configures vars, allocates vals, configs pipelines..

	// this copy is pretty fast -- most of time is below
	pvl, _ := setu.Vars[0].Vals.ValByIdxTry(0)
	pvl.CopyFromBytes(unsafe.Pointer(&UiS1))
	pvl, _ = setu.Vars[1].Vals.ValByIdxTry(0)
	pvl.CopyFromBytes(unsafe.Pointer(&UiS2))
	pvl, _ = setu.Vars[2].Vals.ValByIdxTry(0)
	pvl.CopyFromBytes(unsafe.Pointer(&UiS3))
	pvl, _ = setu.Vars[3].Vals.ValByIdxTry(0)
	pvl.CopyFromBytes(unsafe.Pointer(&UiS4))
	pvl, _ = setu.Vars[4].Vals.ValByIdxTry(0)
	pvl.CopyFromBytes(unsafe.Pointer(&UiSC1))
	pvl, _ = setu.Vars[5].Vals.ValByIdxTry(0)
	pvl.CopyFromBytes(unsafe.Pointer(&UiSC2))
	pvl, _ = setu.Vars[6].Vals.ValByIdxTry(0)
	pvl.CopyFromBytes(unsafe.Pointer(&UiSC3))
	pvl, _ = setu.Vars[7].Vals.ValByIdxTry(0)
	pvl.CopyFromBytes(unsafe.Pointer(&UiSC4))

	rvl, _ := resv.Vals.ValByIdxTry(0)
	rvl.CopyFromBytes(unsafe.Pointer(&res[0]))

	sy.Mem.SyncToGPU()

	vars.BindDynVarsAll()

	sy.CmdResetBindVars(sy.CmdPool.Buff, 0)

	pl.RunComputeWait(sy.CmdPool.Buff, n, 1, 1)
	// note: could use semaphore here instead of waiting on the compute

	sy.Mem.SyncValIdxFmGPU(1, "Res", 0)
	rvl.CopyToBytes(unsafe.Pointer(&res[0]))

	mx := ints.MinInt(n, 14)
	for i := 0; i < mx; i++ {
		d := &res[i]
		fmt.Printf("%d   %d   \n", i, d.Field1)
	}
	fmt.Printf("\n")

	sy.Destroy()
	gp.Destroy()
	vgpu.Terminate()
}
