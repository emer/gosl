// Copyright (c) 2022, The Goki Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/emer/emergent/timer"
	"github.com/goki/ki/ints"
	"github.com/goki/mat32"
	"github.com/goki/vgpu/vgpu"
)

// DiffTol is tolerance on testing diff between cpu and gpu values
const DiffTol = 1.0e-5

// note: standard one to use is plain "gosl" which should be go install'd

//go:generate ../../gosl -exclude=Update,UpdateParams,Defaults -keep github.com/goki/mat32/fastexp.go minmax chans/chans.go chans kinase time.go neuron.go act.go learn.go layer.go axon.hlsl

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
	gp.Config("axon")

	// gp.PropsString(true) // print

	// n := 10 // debugging
	n := 100000 // 1,000,000 = 80x even with range checking
	// 100,000 = ~60x "

	// AMD is 64, NVIDIA, M1 are 32
	threads := 64
	nInt := ints.IntMultiple(n, threads)
	// n = nInt // enforce optimal n's -- otherwise requires range checking
	nGps := nInt / threads // dispatch n

	maxCycles := 200

	lay := &Layer{}
	lay.Defaults()

	time := NewTime()
	time.Defaults()

	neur1 := make([]Neuron, n)
	for i := range neur1 {
		d := &neur1[i]
		lay.Act.InitActs(d)
		d.GeBase = 0.4
	}
	neur2 := make([]Neuron, n)
	for i := range neur2 {
		d := &neur2[i]
		lay.Act.InitActs(d)
		d.GeBase = 0.4
	}

	cpuTmr := timer.Time{}
	cpuTmr.Start()

	for cy := 0; cy < maxCycles; cy++ {
		for i := range neur1 {
			d := &neur1[i]
			// d.Vm = lay.Act.Decay.Glong
			lay.CycleNeuron(i, d, time)
		}
		lay.CycleTimeInc(time)
		// fmt.Printf("%d\ttime.RandCtr: %v\n", cy, time.RandCtr.Uint2())
	}

	cpuTmr.Stop()

	time.Reset()

	sy := gp.NewComputeSystem("axon")
	pl := sy.NewPipeline("axon")
	pl.AddShaderFile("axon", vgpu.ComputeShader, "shaders/axon.spv")

	vars := sy.Vars()
	setl := vars.AddSet()
	sett := vars.AddSet()
	setn := vars.AddSet()

	layv := setl.AddStruct("Layer", int(unsafe.Sizeof(Layer{})), 1, vgpu.Uniform, vgpu.ComputeShader)
	timev := sett.AddStruct("Time", int(unsafe.Sizeof(Time{})), 1, vgpu.Storage, vgpu.ComputeShader)
	neurv := setn.AddStruct("Neurons", int(unsafe.Sizeof(Neuron{})), n, vgpu.Storage, vgpu.ComputeShader)

	setl.ConfigVals(1) // one val per var
	sett.ConfigVals(1) // one val per var
	setn.ConfigVals(1) // one val per var
	sy.Config()        // configures vars, allocates vals, configs pipelines..

	gpuFullTmr := timer.Time{}
	gpuFullTmr.Start()

	// this copy is pretty fast -- most of time is below
	lvl, _ := layv.Vals.ValByIdxTry(0)
	lvl.CopyFromBytes(unsafe.Pointer(lay))
	tvl, _ := timev.Vals.ValByIdxTry(0)
	tvl.CopyFromBytes(unsafe.Pointer(time))
	dvl, _ := neurv.Vals.ValByIdxTry(0)
	dvl.CopyFromBytes(unsafe.Pointer(&neur2[0]))

	// gpuFullTmr := timer.Time{}
	// gpuFullTmr.Start()

	sy.Mem.SyncToGPU()

	vars.BindDynValIdx(0, "Layer", 0)
	vars.BindDynValIdx(1, "Time", 0)
	vars.BindDynValIdx(2, "Neurons", 0)

	sy.CmdResetBindVars(sy.CmdPool.Buff, 0)

	// gpuFullTmr := timer.Time{}
	// gpuFullTmr.Start()

	gpuTmr := timer.Time{}
	gpuTmr.Start()

	// note: it is 2x faster to run the for loop within the shader entirely
	pl.ComputeCommand(nGps, 1, 1)
	sy.ComputeSubmit() // technically should wait, but results are same..
	// if validation mode is on, it complains..
	for cy := 1; cy < maxCycles; cy++ {
		sy.ComputeSubmit() // waiting every time is 10x for 100k
	}
	sy.ComputeWait() // waiting only at end is 13x for 100k

	gpuTmr.Stop()

	sy.Mem.SyncValIdxFmGPU(2, "Neurons", 0) // this is about same as SyncToGPU
	dvl.CopyToBytes(unsafe.Pointer(&neur2[0]))

	gpuFullTmr.Stop()

	mx := ints.MinInt(n, 1)
	anyDiff := false
	for i := 0; i < mx; i++ {
		d1 := &neur1[i]
		d2 := &neur2[i]
		fmt.Printf("\n%14s\t   CPU\t   GPU\tDiff\n", "Var")
		for vi, vn := range NeuronVars {
			v1 := d1.VarByIndex(vi)
			v2 := d2.VarByIndex(vi)
			diff := ""
			if mat32.Abs(v1-v2) > DiffTol {
				diff = "*"
				anyDiff = true
			}
			fmt.Printf("%14s\t%6.4g\t%6.4g\t%s\n", vn, v1, v2, diff)
		}
	}
	fmt.Printf("\n")
	if anyDiff {
		fmt.Printf("ERROR: Differences between CPU and GPU detected -- see stars above\n\n")
	}

	cpu := cpuTmr.TotalSecs()
	gpu := gpuTmr.TotalSecs()
	fmt.Printf("N: %d\t CPU: %6.4g\t GPU: %6.4g\t Full: %6.4g\t CPU/GPU: %6.4g\n", n, cpu, gpu, gpuFullTmr.TotalSecs(), cpu/gpu)

	sy.Destroy()
	gp.Destroy()
	vgpu.Terminate()
}
