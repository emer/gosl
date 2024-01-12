// Copyright (c) 2022, The Goki Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"runtime"
	"unsafe"

	"log/slog"

	"github.com/emer/emergent/v2/timer"
	"github.com/emer/gosl/v2/sltype"
	"github.com/emer/gosl/v2/threading"
	"goki.dev/mat32"
	"goki.dev/vgpu"
)

// DiffTol is tolerance on testing diff between cpu and gpu values
const DiffTol = 1.0e-3

// note: standard one to use is plain "gosl" which should be go install'd

//go:generate ../../gosl -exclude=Update,UpdateParams,Defaults -keep goki.dev/mat32/fastexp.go minmax chans/chans.go chans kinase time.go neuron.go act.go learn.go layer.go axon.hlsl

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

	// n := 64 // debugging
	n := 100000 // 1,000,000 = 80x even with range checking
	// 100,000 = ~60x "

	// AMD is 64, NVIDIA, M1 are 32
	gpuThreads := 64
	cpuThreads := 10
	nInt := int(mat32.IntMultiple(float32(n), float32(gpuThreads)))
	n = nInt                  // enforce optimal n's -- otherwise requires range checking
	nGps := nInt / gpuThreads // dispatch n

	maxCycles := 200 // 70x speedup doing 20000
	// fmt.Printf("n: %d   cycles: %d\n", n, maxCycles)

	nLays := 2
	nfirst := n / nLays
	lays := make([]Layer, nLays)
	for li := range lays {
		ly := &lays[li]
		ly.Defaults()
	}

	time := NewTime()
	time.Defaults()

	neur1 := make([]Neuron, n)
	for i := range neur1 {
		nrn := &neur1[i]
		if i > nfirst {
			nrn.LayIdx = 1
		}
		ly := &lays[nrn.LayIdx]
		ly.Act.InitActs(nrn)
		nrn.GeBase = 0.4
	}
	neur2 := make([]Neuron, n)
	for i := range neur2 {
		nrn := &neur2[i]
		if i > nfirst {
			nrn.LayIdx = 1
		}
		ly := &lays[nrn.LayIdx]
		ly.Act.InitActs(nrn)
		nrn.GeBase = 0.4
	}

	// for testing alignment and buffer type isues
	idxs := make([]sltype.Uint2, n)
	for i := range idxs {
		iv := &idxs[i]
		iv.X = uint32(i)
		iv.Y = uint32(i)
		// iv.Z = uint32(i)
		// iv.W = uint32(i)
	}

	cpuTmr := timer.Time{}
	cpuTmr.Start()

	for cy := 0; cy < maxCycles; cy++ {
		threading.ParallelRun(func(st, ed int) {
			for ni := st; ni < ed; ni++ {
				nrn := &neur1[ni]
				ly := &lays[nrn.LayIdx]
				ly.CycleNeuron(ni, nrn, time)
			}
		}, len(neur1), cpuThreads)
		ly := &lays[0]
		ly.CycleTimeInc(time)
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
	// seti := vars.AddSet()

	// important: Uniform appears to have much higher alignment restrictions
	// compared to Storage -- Layer works but Uint4 does not.
	// Storage however *does* appear to work with only 32 or 16 byte values!
	// all of this is on mac

	layv := setl.AddStruct("Layers", int(unsafe.Sizeof(Layer{})), nLays, vgpu.Storage, vgpu.ComputeShader)
	timev := sett.AddStruct("Time", int(unsafe.Sizeof(Time{})), 1, vgpu.Storage, vgpu.ComputeShader)
	neurv := setn.AddStruct("Neurons", int(unsafe.Sizeof(Neuron{})), n, vgpu.Storage, vgpu.ComputeShader)
	// var ui sltype.Uint2
	// idxv := seti.AddStruct("Idxs", int(unsafe.Sizeof(ui)), n, vgpu.Storage, vgpu.ComputeShader)

	setl.ConfigVals(1) // one val per var
	sett.ConfigVals(1) // one val per var
	setn.ConfigVals(1) // one val per var
	// seti.ConfigVals(1) // one val per var
	sy.Config() // configures vars, allocates vals, configs pipelines..

	gpuFullTmr := timer.Time{}
	gpuFullTmr.Start()

	// this copy is pretty fast -- most of time is below
	lvl, _ := layv.Vals.ValByIdxTry(0)
	lvl.CopyFromBytes(unsafe.Pointer(&lays[0]))
	tvl, _ := timev.Vals.ValByIdxTry(0)
	tvl.CopyFromBytes(unsafe.Pointer(time))
	nvl, _ := neurv.Vals.ValByIdxTry(0)
	nvl.CopyFromBytes(unsafe.Pointer(&neur2[0]))
	// ivl, _ := idxv.Vals.ValByIdxTry(0)
	// ivl.CopyFromBytes(unsafe.Pointer(&idxs[0]))

	// gpuFullTmr := timer.Time{}
	// gpuFullTmr.Start()

	sy.Mem.SyncToGPU()

	vars.BindDynValIdx(0, "Layers", 0)
	vars.BindDynValIdx(1, "Time", 0)
	vars.BindDynValIdx(2, "Neurons", 0)
	// vars.BindDynValIdx(3, "Idxs", 0)

	cmd := sy.ComputeCmdBuff()
	sy.CmdResetBindVars(cmd, 0)

	// gpuFullTmr := timer.Time{}
	// gpuFullTmr.Start()

	gpuTmr := timer.Time{}
	gpuTmr.Start()

	// note: it is 2x faster to run the for loop within the shader entirely
	pl.ComputeDispatch(cmd, nGps, 1, 1)
	sy.ComputeCmdEnd(cmd)
	sy.ComputeSubmitWait(cmd) // technically should wait, but results are same..

	gpuTmr.Stop()

	sy.Mem.SyncValIdxFmGPU(2, "Neurons", 0) // this is about same as SyncToGPU
	nvl.CopyToBytes(unsafe.Pointer(&neur2[0]))

	gpuFullTmr.Stop()

	mx := min(n, 1)
	_ = mx
	anyDiff := false
	// for i := n - 1; i < n; i++ {
	for i := 0; i < 1; i++ {
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
		slog.Error("Differences between CPU and GPU detected -- see stars above\n")
	}

	cpu := cpuTmr.TotalSecs()
	gpu := gpuTmr.TotalSecs()
	fmt.Printf("N: %d\t CPU: %6.4g\t GPU: %6.4g\t Full: %6.4g\t CPU/GPU: %6.4g\n", n, cpu, gpu, gpuFullTmr.TotalSecs(), cpu/gpu)

	sy.Destroy()
	gp.Destroy()
	vgpu.Terminate()
}
