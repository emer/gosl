// Copyright (c) 2020, The Emergent Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package chans

import (
	"cogentcore.org/core/math32"
)

//gosl: start axon

// GABABParams control the GABAB dynamics in PFC Maint neurons,
// based on Brunel & Wang (2001) parameters.
type GABABParams struct {

	// overall strength multiplier of GABA-B current
	Gbar float32 `default:"0,0.2,0.25,0.3,0.4"`

	// rise time for bi-exponential time dynamics of GABA-B
	RiseTau float32 `default:"45"`

	// decay time for bi-exponential time dynamics of GABA-B
	DecayTau float32 `default:"50"`

	// baseline level of GABA-B channels open independent of inhibitory input (is added to spiking-produced conductance)
	Gbase float32 `default:"0.2"`

	// multiplier for converting Gi to equivalent GABA spikes
	GiSpike float32 `default:"10"`

	// time offset when peak conductance occurs, in msec, computed from RiseTau and DecayTau
	MaxTime float32 `edit:"-"`

	// time constant factor used in integration: (Decay / Rise) ^ (Rise / (Decay - Rise))
	TauFact float32 `view:"-"`

	pad float32
}

func (gp *GABABParams) Defaults() {
	gp.Gbar = 0.2
	gp.RiseTau = 45
	gp.DecayTau = 50
	gp.Gbase = 0.2
	gp.GiSpike = 10
	gp.Update()
}

func (gp *GABABParams) Update() {
	gp.TauFact = math32.Pow(gp.DecayTau/gp.RiseTau, gp.RiseTau/(gp.DecayTau-gp.RiseTau))
	gp.MaxTime = ((gp.RiseTau * gp.DecayTau) / (gp.DecayTau - gp.RiseTau)) * math32.Log(gp.DecayTau/gp.RiseTau)
}

// GFromV returns the GABA-B conductance as a function of normalized membrane potential
func (gp *GABABParams) GFromV(v float32) float32 {
	var vbio float32
	vbio = VToBio(v)
	if vbio < -90 {
		vbio = -90
	}
	return 1.0 / (1.0 + math32.FastExp(0.1*((vbio+90)+10)))
}

// GFromS returns the GABA-B conductance as a function of GABA spiking rate,
// based on normalized spiking factor (i.e., Gi from FFFB etc)
func (gp *GABABParams) GFromS(s float32) float32 {
	var ss float32
	ss = s * gp.GiSpike
	if ss > 10 {
		ss = 10
	}
	return 1.0 / (1.0 + math32.FastExp(-(ss-7.1)/1.4))
}

// DG returns dG delta for g
func (gp *GABABParams) DG(g, x float32) float32 {
	return (gp.TauFact*x - g) / gp.RiseTau
}

// DX returns dX delta for x
func (gp *GABABParams) DX(x float32) float32 {
	return -x / gp.DecayTau
}

// GFromGX returns the updated GABA-B / GIRK conductance
// based on current values and gi inhibitory conductance (proxy for GABA spikes)
func (gp *GABABParams) GFromGX(gabaB, gabaBx float32) float32 {
	return gabaB + gp.DG(gabaB, gabaBx)
}

// XFromGiX returns the updated GABA-B x value
// based on current values and gi inhibitory conductance (proxy for GABA spikes)
func (gp *GABABParams) XFromGiX(gabaBx, gi float32) float32 {
	return gabaBx + gp.GFromS(gi) + gp.DX(gabaBx)
}

// GgabaB returns the overall net GABAB / GIRK conductance including
// Gbar, Gbase, and voltage-gating
func (gp *GABABParams) GgabaB(gabaB, vm float32) float32 {
	return gp.Gbar * gp.GFromV(vm) * (gabaB + gp.Gbase)
}

//gosl: end axon
