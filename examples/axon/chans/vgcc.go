// Copyright (c) 2020, The Emergent Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package chans

import (
	"cogentcore.org/core/mat32"
)

//gosl: start axon

// VGCCParams control the standard L-type Ca channel
type VGCCParams struct {

	// strength of VGCC current -- 0.12 value is from Urakubo et al (2008) model -- best fits actual model behavior using axon equations (1.5 nominal in that model), 0.02 works better in practice for not getting stuck in high plateau firing
	Gbar float32 `default:"0.02,0.12"`

	// calcium from conductance factor -- important for learning contribution of VGCC
	Ca float32 `default:"25"`

	pad, pad1 float32
}

func (np *VGCCParams) Defaults() {
	np.Gbar = 0.02
	np.Ca = 25
}

func (np *VGCCParams) Update() {
}

// GFromV returns the VGCC conductance as a function of normalized membrane potential
func (np *VGCCParams) GFromV(v float32) float32 {
	var vbio float32
	vbio = VToBio(v)
	if vbio > -0.1 && vbio < 0.1 {
		return 1.0 / (0.0756 + 0.5*vbio)
	}
	return -vbio / (1.0 - mat32.FastExp(0.0756*vbio))
}

// MFromV returns the M gate function from vbio (not normalized, must not exceed 0)
func (np *VGCCParams) MFromV(vbio float32) float32 {
	if vbio < -60 {
		return 0
	}
	if vbio > -10 {
		return 1
	}
	return 1.0 / (1.0 + mat32.FastExp(-(vbio + 37)))
}

// HFromV returns the H gate function from vbio (not normalized, must not exceed 0)
func (np *VGCCParams) HFromV(vbio float32) float32 {
	if vbio < -50 {
		return 1
	}
	if vbio > -10 {
		return 0
	}
	return 1.0 / (1.0 + mat32.FastExp((vbio+41)*2))
}

// DMHFromV returns the change at msec update scale in M, H factors
// as a function of V normalized (0-1)
func (np *VGCCParams) DMHFromV(v, m, h float32, dm, dh *float32) {
	var vbio float32
	vbio = VToBio(v)
	if vbio > 0 {
		vbio = 0
	}
	*dm = (np.MFromV(vbio) - m) / 3.6
	*dh = (np.HFromV(vbio) - h) / 29.0
}

// Gvgcc returns the VGCC net conductance from m, h activation and vm
func (np *VGCCParams) Gvgcc(vm, m, h float32) float32 {
	return np.Gbar * np.GFromV(vm) * m * m * m * h
}

// CaFromG returns the Ca from Gvgcc conductance, current Ca level,
// and normalized membrane potential.
func (np *VGCCParams) CaFromG(v, g, ca float32) float32 {
	var vbio float32
	vbio = VToBio(v)
	return -vbio * np.Ca * g
}

//gosl: end axon
