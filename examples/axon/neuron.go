// Copyright (c) 2019, The Emergent Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"reflect"
	"unsafe"

	"cogentcore.org/core/math32"
)

//gosl: start axon

// NeuronFlags are bit-flags encoding relevant binary state for neurons
type NeuronFlags int32

// The neuron flags
const (
	// NeuronOff flag indicates that this neuron has been turned off (i.e., lesioned)
	NeuronOff NeuronFlags = 1

	// NeuronHasExt means the neuron has external input in its Ext field
	NeuronHasExt NeuronFlags = 1 << 2

	// NeuronHasTarg means the neuron has external target input in its Target field
	NeuronHasTarg NeuronFlags = 1 << 3

	// NeuronHasCmpr means the neuron has external comparison input in its Target field -- used for computing
	// comparison statistics but does not drive neural activity ever
	NeuronHasCmpr NeuronFlags = 1 << 4
)

// axon.Neuron holds all of the neuron (unit) level variables.
// This is the most basic version, without any optional features.
// All variables accessible via Unit interface must be float32
// and start at the top, in contiguous order
type Neuron struct {

	// bit flags for binary state variables
	Flags NeuronFlags

	// index of the layer that this neuron belongs to -- needed for neuron-level parallel code.
	LayIndex uint32

	// index of the sub-level inhibitory pool that this neuron is in (only for 4D shapes, the pool (unit-group / hypercolumn) structure level) -- indicies start at 1 -- 0 is layer-level pool (is 0 if no sub-pools).
	SubPool int32

	// whether neuron has spiked or not on this cycle (0 or 1)
	Spike float32

	// 1 if neuron has spiked within the last 10 cycles (msecs), corresponding to a nominal max spiking rate of 100 Hz, 0 otherwise -- useful for visualization and computing activity levels in terms of average spiked levels.
	Spiked float32

	// rate-coded activation value reflecting instantaneous estimated rate of spiking, based on 1 / ISIAvg.  This drives feedback inhibition in the FFFB function (todo: this will change when better inhibition is implemented), and is integrated over time for ActInt which is then used for performance statistics and layer average activations, etc.  Should not be used for learning or other computations.
	Act float32

	// integrated running-average activation value computed from Act to produce a longer-term integrated value reflecting the overall activation state across a reasonable time scale to reflect overall response of network to current input state -- this is copied to ActM and ActP at the ends of the minus and plus phases, respectively, and used in computing performance-level statistics (which are typically based on ActM).  Should not be used for learning or other computations.
	ActInt float32

	// ActInt activation state at end of third quarter, representing the posterior-cortical minus phase activation -- used for statistics and monitoring network performance. Should not be used for learning or other computations.
	ActM float32

	// ActInt activation state at end of fourth quarter, representing the posterior-cortical plus_phase activation -- used for statistics and monitoring network performance.  Should not be used for learning or other computations.
	ActP float32

	// external input: drives activation of unit from outside influences (e.g., sensory input)
	Ext float32

	// target value: drives learning to produce this activation value
	Target float32

	// time-integrated total excitatory synaptic conductance, with an instantaneous rise time from each spike (in GeRaw) and exponential decay with Dt.GeTau, aggregated over projections -- does *not* include Gbar.E
	GeSyn float32

	// total excitatory conductance, including all forms of excitation (e.g., NMDA) -- does *not* include Gbar.E
	Ge float32

	// time-integrated total inhibitory synaptic conductance, with an instantaneous rise time from each spike (in GiRaw) and exponential decay with Dt.GiTau, aggregated over projections -- does *not* include Gbar.I.  This is added with computed FFFB inhibition to get the full inhibition in Gi
	GiSyn float32

	// total inhibitory synaptic conductance -- the net inhibitory input to the neuron -- does *not* include Gbar.I
	Gi float32

	// total potassium conductance, typically reflecting sodium-gated potassium currents involved in adaptation effects -- does *not* include Gbar.K
	Gk float32

	// net current produced by all channels -- drives update of Vm
	Inet float32

	// membrane potential -- integrates Inet current over time
	Vm float32

	// dendritic membrane potential -- has a slower time constant, is not subject to the VmR reset after spiking
	VmDend float32

	// spike-driven calcium trace for synapse-level Ca-driven learning: exponential integration of SpikeG * Spike at SynTau time constant (typically 30).  Synapses integrate send.CaSyn * recv.CaSyn across M, P, D time integrals for the synaptic trace driving credit assignment in learning. Time constant reflects binding time of Glu to NMDA and Ca buffering postsynaptically, and determines time window where pre * post spiking must overlap to drive learning.
	CaSyn float32

	// spike-driven calcium trace used as a neuron-level proxy for synpatic credit assignment factor based on time-integrated spiking: exponential integration of SpikeG * Spike at MTau time constant (typically 5).  Simulates a calmodulin (CaM) like signal at the most abstract level.
	CaSpkM float32

	// cascaded integration of CaSpkM at PTau time constant (typically 40), representing neuron-level purely spiking version of plus, LTP direction of weight change and capturing the function of CaMKII in the Kinase learning rule. Used for specialized learning and computational functions, statistics, instead of Act.
	CaSpkP float32

	// cascaded integration CaSpkP at DTau time constant (typically 40), representing neuron-level purely spiking version of minus, LTD direction of weight change and capturing the function of DAPK1 in the Kinase learning rule. Used for specialized learning and computational functions, statistics, instead of Act.
	CaSpkD float32

	// minus-phase snapshot of the CaSpkP value -- similar to ActM but using a more directly spike-integrated value.
	CaSpkPM float32

	// recv neuron calcium signal used to drive temporal error difference component of standard learning rule, combining NMDA (NmdaCa) and spiking-driven VGCC (VgccCaInt) calcium sources (vs. CaSpk* which only reflects spiking component).  This is integrated into CaM, CaP, CaD, and temporal derivative is CaP - CaD (CaMKII - DAPK1).  This approximates the backprop error derivative on net input, but VGCC component adds a proportion of recv activation delta as well -- a balance of both works best.  The synaptic-level trace multiplier provides the credit assignment factor, reflecting coincident activity and potentially integrated over longer multi-trial timescales.
	CaLrn float32

	// integrated CaLrn at MTau timescale (typically 5), simulating a calmodulin (CaM) like signal, which then drives CaP, CaD for delta signal driving error-driven learning.
	CaM float32

	// cascaded integration of CaM at PTau time constant (typically 40), representing the plus, LTP direction of weight change and capturing the function of CaMKII in the Kinase learning rule.
	CaP float32

	// cascaded integratoin of CaP at DTau time constant (typically 40), representing the minus, LTD direction of weight change and capturing the function of DAPK1 in the Kinase learning rule.
	CaD float32

	// difference between CaP - CaD -- this is the error signal that drives error-driven learning.
	CaDiff float32

	// Ca integrated like CaSpkP but only starting at MacCycStart cycle, to prevent inclusion of carryover spiking from prior theta cycle trial -- the PTau time constant otherwise results in significant carryover.
	SpkMaxCa float32

	// maximum CaSpkP across one theta cycle time window -- used for specialized algorithms that have more phasic behavior within a single trial, e.g., BG Matrix layer gating.  Also useful for visualization of peak activity of neurons.
	SpkMax float32

	// final CaSpkD activation state at end of previous theta cycle.  used for specialized learning mechanisms that operate on delayed sending activations.
	SpkPrv float32

	// the activation state at specific time point within current state processing window (e.g., 50 msec for beta cycle within standard theta cycle), as saved by SpkSt1() function.  Used for example in hippocampus for CA3, CA1 learning
	SpkSt1 float32

	// the activation state at specific time point within current state processing window (e.g., 100 msec for beta cycle within standard theta cycle), as saved by SpkSt2() function.  Used for example in hippocampus for CA3, CA1 learning
	SpkSt2 float32

	// recv-unit based learning rate multiplier, reflecting the sigmoid derivative computed from the CaSpkD of recv unit, and the normalized difference CaSpkP - CaSpkD / MAX(CaSpkP - CaSpkD).
	RLRate float32

	// average activation (of minus phase activation state) over long time intervals (time constant = Dt.LongAvgTau) -- useful for finding hog units and seeing overall distribution of activation
	ActAvg float32

	// ActAvg as a proportion of overall layer activation -- this is used for synaptic scaling to match TrgAvg activation -- updated at SlowInterval intervals
	AvgPct float32

	// neuron's target average activation as a proportion of overall layer activation, assigned during weight initialization, driving synaptic scaling relative to AvgPct
	TrgAvg float32

	// change in neuron's target average activation as a result of unit-wise error gradient -- acts like a bias weight.  MPI needs to share these across processors.
	DTrgAvg float32

	// AvgPct - TrgAvg -- i.e., the error in overall activity level relative to set point for this neuron, which drives synaptic scaling -- updated at SlowInterval intervals
	AvgDif float32

	// Attentional modulation factor, which can be set by special layers such as the TRC -- multiplies Ge
	Attn float32

	// current inter-spike-interval -- counts up since last spike.  Starts at -1 when initialized.
	ISI float32

	// average inter-spike-interval -- average time interval between spikes, integrated with ISITau rate constant (relatively fast) to capture something close to an instantaneous spiking rate.  Starts at -1 when initialized, and goes to -2 after first spike, and is only valid after the second spike post-initialization.
	ISIAvg float32

	// accumulating poisson probability factor for driving excitatory noise spiking -- multiply times uniform random deviate at each time step, until it gets below the target threshold based on lambda.
	GeNoiseP float32

	// integrated noise excitatory conductance, added into Ge
	GeNoise float32

	// accumulating poisson probability factor for driving inhibitory noise spiking -- multiply times uniform random deviate at each time step, until it gets below the target threshold based on lambda.
	GiNoiseP float32

	// integrated noise inhibotyr conductance, added into Gi
	GiNoise float32

	// time-averaged Ge value over the minus phase -- useful for stats to set strength of connections etc to get neurons into right range of overall excitatory drive
	GeM float32

	// time-averaged GiSyn value over the minus phase -- useful for stats to set strength of connections etc to get neurons into right range of overall excitatory drive
	GiM float32

	// accumulating voltage-gated gating value for the medium time scale AHP
	MahpN float32

	// slowly accumulating calcium value that drives the slow AHP
	SahpCa float32

	// sAHP gating value
	SahpN float32

	// conductance of sodium-gated potassium channel (KNa) medium dynamics (Slick) -- produces accommodation / adaptation of firing
	GknaMed float32

	// conductance of sodium-gated potassium channel (KNa) slow dynamics (Slack) -- produces accommodation / adaptation of firing
	GknaSlow float32

	// integrated NMDA recv synaptic current -- adds GeRaw and decays with time constant
	GnmdaSyn float32

	// net postsynaptic (recv) NMDA conductance, after Mg V-gating and Gbar -- added directly to Ge as it has the same reversal potential
	Gnmda float32

	// learning version of integrated NMDA recv synaptic current -- adds GeRaw and decays with time constant -- drives NmdaCa that then drives CaM for learning
	GnmdaLrn float32

	// NMDA calcium computed from GnmdaLrn, drives learning via CaM
	NmdaCa float32

	// Sender-based number of open NMDA channels based on spiking activity and consequent glutamate release for all sending synapses -- this is the presynaptic component of NMDA activation that can be used for computing Ca levels for learning -- increases by (1-SnmdaI)*(1-SnmdaO) with spiking and decays otherwise
	SnmdaO float32

	// Sender-based inhibitory factor on NMDA as a function of sending (presynaptic) spiking history, capturing the allosteric dynamics from Urakubo et al (2008) model.  Increases to 1 with every spike, and decays back to 0 with its own longer decay rate.
	SnmdaI float32

	// net GABA-B conductance, after Vm gating and Gbar + Gbase -- applies to Gk, not Gi, for GIRK, with .1 reversal potential.
	GgabaB float32

	// GABA-B / GIRK activation -- time-integrated value with rise and decay time constants
	GABAB float32

	// GABA-B / GIRK internal drive variable -- gets the raw activation and decays
	GABABx float32

	// conductance (via Ca) for VGCC voltage gated calcium channels
	Gvgcc float32

	// activation gate of VGCC channels
	VgccM float32

	// inactivation gate of VGCC channels
	VgccH float32

	// instantaneous VGCC calcium flux -- can be driven by spiking or directly from Gvgcc
	VgccCa float32

	// time-integrated VGCC calcium flux -- this is actually what drives learning
	VgccCaInt float32

	// extra excitatory conductance added to Ge -- from Ext input, deep.GeCtxt etc
	GeExt float32

	// raw excitatory conductance (net input) received from senders = current raw spiking drive
	GeRaw float32

	// baseline level of Ge, added to GeRaw, for intrinsic excitability
	GeBase float32

	// raw inhibitory conductance (net input) received from senders  = current raw spiking drive
	GiRaw float32

	// baseline level of Gi, added to GiRaw, for intrinsic excitability
	GiBase float32

	// SST+ somatostatin positive slow spiking inhibition
	SSGi float32

	// amount of SST+ somatostatin positive slow spiking inhibition applied to dendritic Vm (VmDend)
	SSGiDend float32

	// conductance of A-type K potassium channels
	Gak float32
}

func (nrn *Neuron) HasFlag(flag NeuronFlags) bool {
	return (nrn.Flags & flag) != 0
}

func (nrn *Neuron) SetFlag(flag NeuronFlags) {
	nrn.Flags |= flag
}

func (nrn *Neuron) ClearFlag(flag NeuronFlags) {
	nrn.Flags &^= flag
}

// IsOff returns true if the neuron has been turned off (lesioned)
func (nrn *Neuron) IsOff() bool {
	return nrn.HasFlag(NeuronOff)
}

//gosl: end axon

// NeuronVarStart is the starting field where float32 variables start
// all variables prior must be 32 bit (int32)
// Note: all non-float32 infrastructure variables must be at the start!
const NeuronVarStart = 3

var NeuronVars = []string{}

var NeuronVarsMap map[string]int

var NeuronVarProps = map[string]string{
	"GeSyn":     `range:"2"`,
	"Ge":        `range:"2"`,
	"GeM":       `range:"2"`,
	"Vm":        `min:"0" max:"1"`,
	"VmDend":    `min:"0" max:"1"`,
	"ISI":       `auto-scale:"+"`,
	"ISIAvg":    `auto-scale:"+"`,
	"Gi":        `auto-scale:"+"`,
	"Gk":        `auto-scale:"+"`,
	"ActDel":    `auto-scale:"+"`,
	"ActDiff":   `auto-scale:"+"`,
	"RLRate":    `auto-scale:"+"`,
	"AvgPct":    `range:"2"`,
	"TrgAvg":    `range:"2"`,
	"DTrgAvg":   `auto-scale:"+"`,
	"MahpN":     `auto-scale:"+"`,
	"GknaMed":   `auto-scale:"+"`,
	"GknaSlow":  `auto-scale:"+"`,
	"Gnmda":     `auto-scale:"+"`,
	"GnmdaSyn":  `auto-scale:"+"`,
	"GnmdaLrn":  `auto-scale:"+"`,
	"NmdaCa":    `auto-scale:"+"`,
	"GgabaB":    `auto-scale:"+"`,
	"GABAB":     `auto-scale:"+"`,
	"GABABx":    `auto-scale:"+"`,
	"Gvgcc":     `auto-scale:"+"`,
	"VgccCa":    `auto-scale:"+"`,
	"VgccCaInt": `auto-scale:"+"`,
	"Gak":       `auto-scale:"+"`,
	"SSGi":      `auto-scale:"+"`,
	"SSGiDend":  `auto-scale:"+"`,
}

func init() {
	NeuronVarsMap = make(map[string]int, len(NeuronVars))
	typ := reflect.TypeOf((*Neuron)(nil)).Elem()
	nf := typ.NumField()
	startIndex := NeuronVarStart
	for i := startIndex; i < nf; i++ {
		fs := typ.FieldByIndex([]int{i})
		v := fs.Name
		NeuronVars = append(NeuronVars, v)
		NeuronVarsMap[v] = i - startIndex
		pstr := NeuronVarProps[v]
		if fld, has := typ.FieldByName(v); has {
			if desc, ok := fld.Tag.Lookup("desc"); ok {
				pstr += ` desc:"` + desc + `"`
				NeuronVarProps[v] = pstr
			}
		}
	}
}

func (nrn *Neuron) VarNames() []string {
	return NeuronVars
}

// NeuronVarIndexByName returns the index of the variable in the Neuron, or error
func NeuronVarIndexByName(varNm string) (int, error) {
	i, ok := NeuronVarsMap[varNm]
	if !ok {
		return -1, fmt.Errorf("Neuron VarByName: variable name: %v not valid", varNm)
	}
	return i, nil
}

// VarByIndex returns variable using index (0 = first variable in NeuronVars list)
func (nrn *Neuron) VarByIndex(idx int) float32 {
	fv := (*float32)(unsafe.Pointer(uintptr(unsafe.Pointer(nrn)) + uintptr(NeuronVarStart*4+4*idx)))
	return *fv
}

// VarByName returns variable by name, or error
func (nrn *Neuron) VarByName(varNm string) (float32, error) {
	i, err := NeuronVarIndexByName(varNm)
	if err != nil {
		return math32.NaN(), err
	}
	return nrn.VarByIndex(i), nil
}
