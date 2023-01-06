// Copyright (c) 2019, The Emergent Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "github.com/goki/gosl/slbool"

//gosl: start axon

// axon.Layer implements the basic Axon spiking activation function,
// and manages learning in the projections.
type Layer struct {
	Act   ActParams       `view:"add-fields" desc:"Activation parameters and methods for computing activations"`
	Learn LearnNeurParams `view:"add-fields" desc:"Learning parameters and methods that operate at the neuron level"`
}

func (ly *Layer) Defaults() {
	ly.Act.Defaults()
	ly.Learn.Defaults()
	ly.Act.Clamp.Ge = 1.5
	ly.Learn.TrgAvgAct.SubMean = 0
}

// todo: why is this UpdateParams and not just Update()?

// UpdateParams updates all params given any changes that might have been made to individual values
// including those in the receiving projections of this layer
func (ly *Layer) UpdateParams() {
	ly.Act.Update()
	ly.Learn.Update()
}

//////////////////////////////////////////////////////////////////////////////////////
//  Cycle

// GiInteg adds Gi values from all sources including Pool computed inhib
// and updates GABAB as well
func (ly *Layer) GiInteg(ni int, nrn *Neuron, ctime *Time) {
	nrn.Gi = nrn.GiSyn + nrn.GiNoise
	nrn.SSGiDend = ly.Act.Dend.SSGi
	nrn.GABAB = ly.Act.GABAB.GFmGX(nrn.GABAB, nrn.GABABx)
	nrn.GABABx = ly.Act.GABAB.XFmGiX(nrn.GABABx, nrn.Gi)
	nrn.GgabaB = ly.Act.GABAB.GgabaB(nrn.GABAB, nrn.VmDend)
	nrn.Gk += nrn.GgabaB // Gk was already init
}

// GFmSpikeRaw integrates G*Raw and G*Syn values for given neuron
// from the Prjn-level GSyn integrated values.
func (ly *Layer) GFmSpikeRaw(ni int, nrn *Neuron, ctime *Time) {
	nrn.GeRaw = 0
	nrn.GiRaw = 0
	nrn.GeSyn = nrn.GeBase
	nrn.GiSyn = nrn.GiBase
	nrn.GeSyn = nrn.GeRaw
}

// GFmRawSyn computes overall Ge and GiSyn conductances for neuron
// from GeRaw and GeSyn values, including NMDA, VGCC, AMPA, and GABA-A channels.
func (ly *Layer) GFmRawSyn(ni int, nrn *Neuron, ctime *Time) {
	ly.Act.NMDAFmRaw(nrn, nrn.GeRaw)
	ly.Learn.LrnNMDAFmRaw(nrn, nrn.GeRaw)
	ly.Act.GvgccFmVm(nrn)
	ly.Act.GeFmSyn(nrn, nrn.GeSyn, nrn.Gnmda+nrn.Gvgcc) // sets nrn.GeExt too
	ly.Act.GkFmVm(nrn)
	nrn.GiSyn = ly.Act.GiFmSyn(nrn, nrn.GiSyn)
}

// GInteg integrates conductances G over time (Ge, NMDA, etc).
// reads pool Gi values
func (ly *Layer) GInteg(ni int, nrn *Neuron, ctime *Time) {
	ly.GFmSpikeRaw(ni, nrn, ctime)
	// note: can add extra values to GeRaw and GeSyn here
	ly.GFmRawSyn(ni, nrn, ctime)
	ly.GiInteg(ni, nrn, ctime)
}

// SpikeFmG computes Vm from Ge, Gi, Gl conductances and then Spike from that
func (ly *Layer) SpikeFmG(ni int, nrn *Neuron, ctime *Time) {
	intdt := ly.Act.Dt.IntDt
	if slbool.IsTrue(ctime.PlusPhase) {
		intdt *= 3.0
	}
	ly.Act.VmFmG(nrn)
	ly.Act.SpikeFmVm(nrn)
	ly.Learn.CaFmSpike(nrn)
	if ctime.Cycle >= ly.Act.Dt.MaxCycStart {
		nrn.SpkMaxCa += ly.Learn.CaSpk.Dt.PDt * (nrn.CaSpkM - nrn.SpkMaxCa)
		if nrn.SpkMaxCa > nrn.SpkMax {
			nrn.SpkMax = nrn.SpkMaxCa
		}
	}
	nrn.ActInt += intdt * (nrn.Act - nrn.ActInt) // using reg act here now
	if slbool.IsFalse(ctime.PlusPhase) {
		nrn.GeM += ly.Act.Dt.IntDt * (nrn.Ge - nrn.GeM)
		nrn.GiM += ly.Act.Dt.IntDt * (nrn.GiSyn - nrn.GiM)
	}
}

// CycleNeuron does one cycle (msec) of updating at the neuron level
func (ly *Layer) CycleNeuron(ni int, nrn *Neuron, ctime *Time) {
	ly.GInteg(ni, nrn, ctime)
	ly.SpikeFmG(ni, nrn, ctime)
}

//gosl: end axon

//gosl: hlsl axon
/*
// // note: double-commented lines required here -- binding is var, set
[[vk::binding(0, 0)]] uniform Layer Lay;
[[vk::binding(0, 1)]] RWStructuredBuffer<Time> time;
[[vk::binding(1, 1)]] RWStructuredBuffer<Neuron> Neurons;
[numthreads(1, 1, 1)]
void main(uint3 idx : SV_DispatchThreadID) {
	Neurons[idx.x].Vm = Lay.Act.Decay.Glong;
    // // Lay.CycleNeuron(idx.x, Neurons[idx.x], time[0]);
}
*/
//gosl: end axon
