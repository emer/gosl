// Copyright (c) 2022, The Emergent Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kinase

//gosl: start axon

// CaDtParams has rate constants for integrating Ca calcium
// at different time scales, including final CaP = CaMKII and CaD = DAPK1
// timescales for LTP potentiation vs. LTD depression factors.
type CaDtParams struct {

	// CaM (calmodulin) time constant in cycles (msec) -- for synaptic-level integration this integrates on top of Ca signal from send->CaSyn * recv->CaSyn, each of which are typically integrated with a 30 msec Tau.
	MTau float32 `default:"2,5" min:"1"`

	// LTP spike-driven Ca factor (CaP) time constant in cycles (msec), simulating CaMKII in the Kinase framework, with 40 on top of MTau roughly tracking the biophysical rise time.  Computationally, CaP represents the plus phase learning signal that reflects the most recent past information.
	PTau float32 `default:"40" min:"1"`

	// LTD spike-driven Ca factor (CaD) time constant in cycles (msec), simulating DAPK1 in Kinase framework.  Computationally, CaD represents the minus phase learning signal that reflects the expectation representation prior to experiencing the outcome (in addition to the outcome).
	DTau float32 `default:"40" min:"1"`

	// rate = 1 / tau
	MDt float32 `view:"-" json:"-" xml:"-" edit:"-"`

	// rate = 1 / tau
	PDt float32 `view:"-" json:"-" xml:"-" edit:"-"`

	// rate = 1 / tau
	DDt float32 `view:"-" json:"-" xml:"-" edit:"-"`

	pad, pad1 float32
}

func (kp *CaDtParams) Defaults() {
	kp.MTau = 5
	kp.PTau = 40
	kp.DTau = 40
	kp.Update()
}

func (kp *CaDtParams) Update() {
	kp.MDt = 1 / kp.MTau
	kp.PDt = 1 / kp.PTau
	kp.DDt = 1 / kp.DTau
}

// CaParams has rate constants for integrating spike-driven Ca calcium
// at different time scales, including final CaP = CaMKII and CaD = DAPK1
// timescales for LTP potentiation vs. LTD depression factors.
type CaParams struct {

	// spiking gain factor for SynSpk learning rule variants.  This alters the overall range of values, keeping them in roughly the unit scale, and affects effective learning rate.
	SpikeG float32 `default:"12"`

	// IMPORTANT: only used for SynSpkTheta learning mode: threshold on Act value for updating synapse-level Ca values -- this is purely a performance optimization that excludes random infrequent spikes -- 0.05 works well on larger networks but not smaller, which require the .01 default.
	UpdateThr float32 `default:"0.01,0.02,0.5"`

	// maximum ISI for integrating in Opt mode -- above that just set to 0
	MaxISI int32 `default:"100"`

	pad float32

	// time constants for integrating at M, P, and D cascading levels
	Dt CaDtParams `view:"inline"`
}

func (kp *CaParams) Defaults() {
	kp.SpikeG = 12
	kp.UpdateThr = 0.01
	kp.MaxISI = 100
	kp.Dt.Defaults()
	kp.Update()
}

func (kp *CaParams) Update() {
	kp.Dt.Update()
}

// FromSpike computes updates to CaM, CaP, CaD from current spike value.
// The SpikeG factor determines strength of increase to CaM.
func (kp *CaParams) FromSpike(spike float32, caM, caP, caD *float32) {
	*caM += kp.Dt.MDt * (kp.SpikeG*spike - *caM)
	*caP += kp.Dt.PDt * (*caM - *caP)
	*caD += kp.Dt.DDt * (*caP - *caD)
}

// FromCa computes updates to CaM, CaP, CaD from current calcium level.
// The SpikeG factor is NOT applied to Ca and should be pre-applied
// as appropriate.
func (kp *CaParams) FromCa(ca float32, caM, caP, caD *float32) {
	*caM += kp.Dt.MDt * (ca - *caM)
	*caP += kp.Dt.PDt * (*caM - *caP)
	*caD += kp.Dt.DDt * (*caP - *caD)
}

// IntFromTime returns the interval from current time
// and last update time, which is -1 if never updated
// (in which case return is -1)
func (kp *CaParams) IntFromTime(ctime, utime int32) int32 {
	if utime < 0 {
		return -1
	}
	return ctime - utime
}

// CurCa updates the current Ca* values, dealing with updating for
// optimized spike-time update versions.
// ctime is current time in msec, and utime is last update time (-1 if never)
func (kp *CaParams) CurCa(ctime, utime int32, caM, caP, caD *float32) {
	isi := kp.IntFromTime(ctime, utime)
	if isi <= 0 {
		return
	}
	if isi > kp.MaxISI {
		*caM = 0
		*caP = 0
		*caD = 0
		return
	}
	for i := int32(0); i < isi; i++ {
		kp.FromCa(0, caM, caP, caD) // just decay to 0
	}
}

//gosl: end axon
