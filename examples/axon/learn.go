// Copyright (c) 2019, The Emergent Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"cogentcore.org/core/mat32"
	"github.com/emer/gosl/v2/examples/axon/chans"
	"github.com/emer/gosl/v2/examples/axon/kinase"
	"github.com/emer/gosl/v2/examples/axon/minmax"
	"github.com/emer/gosl/v2/slbool"
)

///////////////////////////////////////////////////////////////////////
//  learn.go contains the learning params and functions for axon

//gosl: start axon

// CaLrnParams parameterizes the neuron-level calcium signals driving learning:
// CaLrn = NMDA + VGCC Ca sources, where VGCC can be simulated from spiking or
// use the more complex and dynamic VGCC channel directly.
// CaLrn is then integrated in a cascading manner at multiple time scales:
// CaM (as in calmodulin), CaP (ltP, CaMKII, plus phase), CaD (ltD, DAPK1, minus phase).
type CaLrnParams struct {

	// denomenator used for normalizing CaLrn, so the max is roughly 1 - 1.5 or so, which works best in terms of previous standard learning rules, and overall learning performance
	Norm float32 `default:"80"`

	// use spikes to generate VGCC instead of actual VGCC current -- see SpkVGCCa for calcium contribution from each spike
	SpkVGCC slbool.Bool `default:"true"`

	// multiplier on spike for computing Ca contribution to CaLrn in SpkVGCC mode
	SpkVgccCa float32 `default:"35"`

	// time constant of decay for VgccCa calcium -- it is highly transient around spikes, so decay and diffusion factors are more important than for long-lasting NMDA factor.  VgccCa is integrated separately int VgccCaInt prior to adding into NMDA Ca in CaLrn
	VgccTau float32 `default:"10"`

	// time constants for integrating CaLrn across M, P and D cascading levels
	Dt kinase.CaDtParams `view:"inline"`

	// rate = 1 / tau
	VgccDt float32 `view:"-" json:"-" xml:"-" edit:"-"`

	// = 1 / Norm
	NormInv float32 `view:"-" json:"-" xml:"-" edit:"-"`

	pad, pad1 float32
}

func (np *CaLrnParams) Defaults() {
	np.Norm = 80
	np.SpkVGCC = slbool.True
	np.SpkVgccCa = 35
	np.VgccTau = 10
	np.Dt.Defaults()
	np.Dt.MTau = 2
	np.Update()
}

func (np *CaLrnParams) Update() {
	np.Dt.Update()
	np.VgccDt = 1 / np.VgccTau
	np.NormInv = 1 / np.Norm
}

// VgccCa updates the simulated VGCC calcium from spiking, if that option is selected,
// and performs time-integration of VgccCa
func (np *CaLrnParams) VgccCa(nrn *Neuron) {
	if slbool.IsTrue(np.SpkVGCC) {
		nrn.VgccCa = np.SpkVgccCa * nrn.Spike
	}
	nrn.VgccCaInt += nrn.VgccCa - np.VgccDt*nrn.VgccCaInt // Dt only affects decay, not rise time
}

// CaLrn updates the CaLrn value and its cascaded values, based on NMDA, VGCC Ca
// it first calls VgccCa to update the spike-driven version of that variable, and
// perform its time-integration.
func (np *CaLrnParams) CaLrn(nrn *Neuron) {
	np.VgccCa(nrn)
	nrn.CaLrn = np.NormInv * (nrn.NmdaCa + nrn.VgccCaInt)
	nrn.CaM += np.Dt.MDt * (nrn.CaLrn - nrn.CaM)
	nrn.CaP += np.Dt.PDt * (nrn.CaM - nrn.CaP)
	nrn.CaD += np.Dt.DDt * (nrn.CaP - nrn.CaD)
	nrn.CaDiff = nrn.CaP - nrn.CaD
}

// CaSpkParams parameterizes the neuron-level spike-driven calcium
// signals, starting with CaSyn that is integrated at the neuron level
// and drives synapse-level, pre * post Ca integration, which provides the Tr
// trace that multiplies error signals, and drives learning directly for Target layers.
// CaSpk* values are integrated separately at the Neuron level and used for UpdtThr
// and RLRate as a proxy for the activation (spiking) based learning signal.
type CaSpkParams struct {

	// gain multiplier on spike for computing CaSpk: increasing this directly affects the magnitude of the trace values, learning rate in Target layers, and other factors that depend on CaSpk values: RLRate, UpdtThr.  Prjn.KinaseCa.SpikeG provides an additional gain factor specific to the synapse-level trace factors, without affecting neuron-level CaSpk values.  Larger networks require higher gain factors at the neuron level -- 12, vs 8 for smaller.
	SpikeG float32 `default:"8,12"`

	// time constant for integrating spike-driven calcium trace at sender and recv neurons, CaSyn, which then drives synapse-level integration of the joint pre * post synapse-level activity, in cycles (msec)
	SynTau float32 `default:"30" min:"1"`

	// rate = 1 / tau
	SynDt float32 `view:"-" json:"-" xml:"-" edit:"-"`

	// Ca gain factor for SynSpkCa learning rule, to compensate for the effect of SynTau, which increases Ca as it gets larger.  is 1 for SynTau = 30 -- todo: eliminate this at some point!
	SynSpkG float32 `view:"+" json:"-" xml:"-" edit:"-"`

	// time constants for integrating CaSpk across M, P and D cascading levels -- these are typically the same as in CaLrn and Prjn level for synaptic integration, except for the M factor.
	Dt kinase.CaDtParams `view:"inline"`
}

func (np *CaSpkParams) Defaults() {
	np.SpikeG = 8
	np.SynTau = 30
	np.Dt.Defaults()
	np.Update()
}

func (np *CaSpkParams) Update() {
	np.Dt.Update()
	np.SynDt = 1 / np.SynTau
	np.SynSpkG = mat32.Sqrt(30) / mat32.Sqrt(np.SynTau)
}

// CaFmSpike computes CaSpk* and CaSyn calcium signals based on current spike.
func (np *CaSpkParams) CaFmSpike(nrn *Neuron) {
	nsp := np.SpikeG * nrn.Spike
	nrn.CaSyn += np.SynDt * (nsp - nrn.CaSyn)
	nrn.CaSpkM += np.Dt.MDt * (nsp - nrn.CaSpkM)
	nrn.CaSpkP += np.Dt.PDt * (nrn.CaSpkM - nrn.CaSpkP)
	nrn.CaSpkD += np.Dt.DDt * (nrn.CaSpkP - nrn.CaSpkD)
}

//////////////////////////////////////////////////////////////////////////////////////
//  TrgAvgActParams

// TrgAvgActParams govern the target and actual long-term average activity in neurons.
// Target value is adapted by neuron-wise error and difference in actual vs. target.
// drives synaptic scaling at a slow timescale (Network.SlowInterval).
type TrgAvgActParams struct {

	// whether to use target average activity mechanism to scale synaptic weights
	On slbool.Bool

	// learning rate for adjustments to Trg value based on unit-level error signal.  Population TrgAvg values are renormalized to fixed overall average in TrgRange. Generally, deviating from the default doesn't make much difference.
	ErrLRate float32 `viewif:"On" default:"0.02"`

	// rate parameter for how much to scale synaptic weights in proportion to the AvgDif between target and actual proportion activity -- this determines the effective strength of the constraint, and larger models may need more than the weaker default value.
	SynScaleRate float32 `viewif:"On" default:"0.005,0.0002"`

	// amount of mean trg change to subtract -- 1 = full zero sum.  1 works best in general -- but in some cases it may be better to start with 0 and then increase using network SetSubMean method at a later point.
	SubMean float32 `viewif:"On" default:"0,1"`

	// permute the order of TrgAvg values within layer -- otherwise they are just assigned in order from highest to lowest for easy visualization -- generally must be true if any topographic weights are being used
	Permute slbool.Bool `viewif:"On" default:"true"`

	// use pool-level target values if pool-level inhibition and 4D pooled layers are present -- if pool sizes are relatively small, then may not be useful to distribute targets just within pool
	Pool slbool.Bool `viewif:"On"`

	pad, pad1 float32

	// range of target normalized average activations -- individual neurons are assigned values within this range to TrgAvg, and clamped within this range.
	TrgRange minmax.F32 `viewif:"On" default:"{0.5 2}"`
}

func (ta *TrgAvgActParams) Update() {
}

func (ta *TrgAvgActParams) Defaults() {
	ta.On = slbool.True
	ta.ErrLRate = 0.02
	ta.SynScaleRate = 0.005
	ta.SubMean = 1 // 1 in general beneficial
	ta.TrgRange.Set(0.5, 2)
	ta.Permute = slbool.True
	ta.Pool = slbool.True
	ta.Update()
}

//////////////////////////////////////////////////////////////////////////////////////
//  RLRateParams

// RLRateParams are recv neuron learning rate modulation parameters.
// Has two factors: the derivative of the sigmoid based on CaSpkD
// activity levels, and based on the phase-wise differences in activity (Diff).
type RLRateParams struct {

	// use learning rate modulation
	On slbool.Bool `default:"true"`

	// minimum learning rate multiplier for sigmoidal act (1-act) factor -- prevents lrate from going too low for extreme values.  Set to 1 to disable Sigmoid derivative factor, which is default for Target layers.
	SigmoidMin float32 `default:"0.05,1"`

	// modulate learning rate as a function of plus - minus differences
	Diff slbool.Bool

	// threshold on Max(CaSpkP, CaSpkD) below which Min lrate applies -- must be > 0 to prevent div by zero
	SpkThr float32 `default:"0.1"`

	// threshold on recv neuron error delta, i.e., |CaSpkP - CaSpkD| below which lrate is at Min value
	DiffThr float32 `default:"0.02"`

	// for Diff component, minimum learning rate value when below ActDiffThr
	Min float32 `default:"0.001"`

	pad, pad1 float32
}

func (rl *RLRateParams) Update() {
}

func (rl *RLRateParams) Defaults() {
	rl.On = slbool.True
	rl.SigmoidMin = 0.05
	rl.Diff = slbool.True
	rl.SpkThr = 0.1
	rl.DiffThr = 0.02
	rl.Min = 0.001
	rl.Update()
}

// RLRateSigDeriv returns the sigmoid derivative learning rate
// factor as a function of spiking activity, with mid-range values having
// full learning and extreme values a reduced learning rate:
// deriv = act * (1 - act)
// The activity should be CaSpkP and the layer maximum is used
// to normalize that to a 0-1 range.
func (rl *RLRateParams) RLRateSigDeriv(act float32, laymax float32) float32 {
	if slbool.IsFalse(rl.On) || laymax == 0 {
		return 1.0
	}
	ca := act / laymax
	lr := 4.0 * ca * (1 - ca) // .5 * .5 = .25 = peak
	if lr < rl.SigmoidMin {
		lr = rl.SigmoidMin
	}
	return lr
}

// RLRateDiff returns the learning rate as a function of difference between
// CaSpkP and CaSpkD values
func (rl *RLRateParams) RLRateDiff(scap, scad float32) float32 {
	if slbool.IsFalse(rl.On) || slbool.IsFalse(rl.Diff) {
		return 1.0
	}
	mx := mat32.Max(scap, scad)
	if mx > rl.SpkThr { // avoid div by 0
		dif := mat32.Abs(scap - scad)
		if dif < rl.DiffThr {
			return rl.Min
		}
		return (dif / mx)
	}
	return rl.Min
}

// axon.LearnNeurParams manages learning-related parameters at the neuron-level.
// This is mainly the running average activations that drive learning
type LearnNeurParams struct {

	// parameterizes the neuron-level calcium signals driving learning: CaLrn = NMDA + VGCC Ca sources, where VGCC can be simulated from spiking or use the more complex and dynamic VGCC channel directly.  CaLrn is then integrated in a cascading manner at multiple time scales: CaM (as in calmodulin), CaP (ltP, CaMKII, plus phase), CaD (ltD, DAPK1, minus phase).
	CaLrn CaLrnParams `view:"inline"`

	// parameterizes the neuron-level spike-driven calcium signals, starting with CaSyn that is integrated at the neuron level, and drives synapse-level, pre * post Ca integration, which provides the Tr trace that multiplies error signals, and drives learning directly for Target layers. CaSpk* values are integrated separately at the Neuron level and used for UpdtThr and RLRate as a proxy for the activation (spiking) based learning signal.
	CaSpk CaSpkParams `view:"inline"`

	// NMDA channel parameters used for learning, vs. the ones driving activation -- allows exploration of learning parameters independent of their effects on active maintenance contributions of NMDA, and may be supported by different receptor subtypes
	LrnNMDA chans.NMDAParams `view:"inline"`

	// synaptic scaling parameters for regulating overall average activity compared to neuron's own target level
	TrgAvgAct TrgAvgActParams `view:"inline"`

	// recv neuron learning rate modulation params -- an additional error-based modulation of learning for receiver side: RLRate = |SpkCaP - SpkCaD| / Max(SpkCaP, SpkCaD)
	RLRate RLRateParams `view:"inline"`
}

func (ln *LearnNeurParams) Update() {
	ln.CaLrn.Update()
	ln.CaSpk.Update()
	ln.LrnNMDA.Update()
	ln.TrgAvgAct.Update()
	ln.RLRate.Update()
}

func (ln *LearnNeurParams) Defaults() {
	ln.CaLrn.Defaults()
	ln.CaSpk.Defaults()
	ln.LrnNMDA.Defaults()
	ln.LrnNMDA.ITau = 1
	ln.LrnNMDA.Update()
	ln.TrgAvgAct.Defaults()
	ln.RLRate.Defaults()
}

// InitCaLrnSpk initializes the neuron-level calcium learning and spking variables.
// Called by InitWts (at start of learning).
func (ln *LearnNeurParams) InitNeurCa(nrn *Neuron) {
	nrn.GnmdaLrn = 0
	nrn.NmdaCa = 0
	nrn.SnmdaO = 0
	nrn.SnmdaI = 0

	nrn.VgccCa = 0
	nrn.VgccCaInt = 0

	nrn.CaLrn = 0

	nrn.CaSyn = 0
	nrn.CaSpkM = 0
	nrn.CaSpkP = 0
	nrn.CaSpkD = 0
	nrn.CaSpkPM = 0

	nrn.CaM = 0
	nrn.CaP = 0
	nrn.CaD = 0
	nrn.CaDiff = 0
}

// DecayNeurCa decays neuron-level calcium learning and spiking variables
// by given factor.  Note: this is NOT called by default and is generally
// not useful, causing variability in these learning factors as a function
// of the decay parameter that then has impacts on learning rates etc.
// It is only here for reference or optional testing.
func (ln *LearnNeurParams) DecayCaLrnSpk(nrn *Neuron, decay float32) {
	nrn.GnmdaLrn -= decay * nrn.GnmdaLrn
	nrn.NmdaCa -= decay * nrn.NmdaCa
	nrn.SnmdaO -= decay * nrn.SnmdaO
	nrn.SnmdaI -= decay * nrn.SnmdaI

	nrn.VgccCa -= decay * nrn.VgccCa
	nrn.VgccCaInt -= decay * nrn.VgccCaInt

	nrn.CaLrn -= decay * nrn.CaLrn

	nrn.CaSyn -= decay * nrn.CaSyn
	nrn.CaSpkM -= decay * nrn.CaSpkM
	nrn.CaSpkP -= decay * nrn.CaSpkP
	nrn.CaSpkD -= decay * nrn.CaSpkD

	nrn.CaM -= decay * nrn.CaM
	nrn.CaP -= decay * nrn.CaP
	nrn.CaD -= decay * nrn.CaD
}

// LrnNMDAFmRaw updates the separate NMDA conductance and calcium values
// based on GeTot = GeRaw + external ge conductance.  These are the variables
// that drive learning -- can be the same as activation but also can be different
// for testing learning Ca effects independent of activation effects.
func (ln *LearnNeurParams) LrnNMDAFmRaw(nrn *Neuron, geTot float32) {
	if geTot < 0 {
		geTot = 0
	}
	nrn.GnmdaLrn = ln.LrnNMDA.NMDASyn(nrn.GnmdaLrn, geTot)
	gnmda := ln.LrnNMDA.Gnmda(nrn.GnmdaLrn, nrn.VmDend)
	nrn.NmdaCa = gnmda * ln.LrnNMDA.CaFmV(nrn.VmDend)
	ln.LrnNMDA.SnmdaFmSpike(nrn.Spike, &nrn.SnmdaO, &nrn.SnmdaI)
}

// CaFmSpike updates all spike-driven calcium variables, including CaLrn and CaSpk.
// Computed after new activation for current cycle is updated.
func (ln *LearnNeurParams) CaFmSpike(nrn *Neuron) {
	ln.CaSpk.CaFmSpike(nrn)
	ln.CaLrn.CaLrn(nrn)
}

//gosl: end axon
