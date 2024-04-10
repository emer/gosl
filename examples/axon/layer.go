// Copyright (c) 2019, The Emergent Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/emer/gosl/v2/slbool"
	"github.com/emer/gosl/v2/sltype"
)

//gosl: start axon

// axon.Layer implements the basic Axon spiking activation function,
// and manages learning in the projections.
type Layer struct {

	// Activation parameters and methods for computing activations
	Act ActParams `view:"add-fields"`

	// Learning parameters and methods that operate at the neuron level
	Learn LearnNeurParams `view:"add-fields"`
}

func (ly *Layer) Defaults() {
	ly.Act.Defaults()
	ly.Learn.Defaults()
	ly.Act.Clamp.Ge = 1.5
	ly.Learn.TrgAvgAct.SubMean = 0
	ly.Act.Noise.On = slbool.True
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
	nrn.GABAB = ly.Act.GABAB.GFromGX(nrn.GABAB, nrn.GABABx)
	nrn.GABABx = ly.Act.GABAB.XFromGiX(nrn.GABABx, nrn.Gi)
	nrn.GgabaB = ly.Act.GABAB.GgabaB(nrn.GABAB, nrn.VmDend)
	nrn.Gk += nrn.GgabaB // Gk was already init
}

// GFromSpikeRaw integrates G*Raw and G*Syn values for given neuron
// from the Prjn-level GSyn integrated values.
func (ly *Layer) GFromSpikeRaw(ni int, nrn *Neuron, ctime *Time) {
	nrn.GeRaw = 0.4
	nrn.GiRaw = 0
	nrn.GeSyn = nrn.GeBase
	nrn.GiSyn = nrn.GiBase
	nrn.GeSyn = nrn.GeRaw
}

// GFromRawSyn computes overall Ge and GiSyn conductances for neuron
// from GeRaw and GeSyn values, including NMDA, VGCC, AMPA, and GABA-A channels.
func (ly *Layer) GFromRawSyn(ni int, nrn *Neuron, ctime *Time, randctr *sltype.Uint2) {
	ly.Act.NMDAFromRaw(nrn, nrn.GeRaw)
	ly.Learn.LrnNMDAFromRaw(nrn, nrn.GeRaw)
	ly.Act.GvgccFromVm(nrn)
	ly.Act.GeFromSyn(ni, nrn, nrn.GeSyn, nrn.Gnmda+nrn.Gvgcc, randctr) // sets nrn.GeExt too
	ly.Act.GkFromVm(nrn)
	nrn.GiSyn = ly.Act.GiFromSyn(ni, nrn, nrn.GiSyn, randctr)
}

// GInteg integrates conductances G over time (Ge, NMDA, etc).
// reads pool Gi values
func (ly *Layer) GInteg(ni int, nrn *Neuron, ctime *Time, randctr *sltype.Uint2) {
	ly.GFromSpikeRaw(ni, nrn, ctime)
	// note: can add extra values to GeRaw and GeSyn here
	ly.GFromRawSyn(ni, nrn, ctime, randctr)
	ly.GiInteg(ni, nrn, ctime)
}

// SpikeFromG computes Vm from Ge, Gi, Gl conductances and then Spike from that
func (ly *Layer) SpikeFromG(ni int, nrn *Neuron, ctime *Time) {
	intdt := ly.Act.Dt.IntDt
	if slbool.IsTrue(ctime.PlusPhase) {
		intdt *= 3.0
	}
	ly.Act.VmFromG(nrn)
	ly.Act.SpikeFromVm(nrn)
	ly.Learn.CaFromSpike(nrn)
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
	randctr := ctime.RandCtr.Uint2() // use local var
	ly.GInteg(ni, nrn, ctime, &randctr)
	ly.SpikeFromG(ni, nrn, ctime)
}

func (ly *Layer) CycleTimeInc(ctime *Time) {
	ctime.CycleInc()
	ctime.RandCtr.Add(2) // main code uses fixed inc across all layers..
}

//gosl: end axon
