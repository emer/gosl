// Copyright (c) 2019, The Emergent Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

//gosl: start axon

// axon.Time contains all the timing state and parameter information for running a model.
// Can also include other relevant state context, e.g., Testing vs. Training modes.
type Time struct {
	Phase      int     `desc:"phase counter: typicaly 0-1 for minus-plus but can be more phases for other algorithms"`
	PlusPhase  bool    `desc:"true if this is the plus phase, when the outcome / bursting is occurring, driving positive learning -- else minus phase"`
	PhaseCycle int     `desc:"cycle within current phase -- minus or plus"`
	Cycle      int     `desc:"cycle counter: number of iterations of activation updating (settling) on the current state -- this counts time sequentially until reset with NewState"`
	CycleTot   int     `desc:"total cycle count -- this increments continuously from whenever it was last reset -- typically this is number of milliseconds in simulation time"`
	Time       float32 `desc:"accumulated amount of time the network has been running, in simulation-time (not real world time), in seconds"`
	Mode       string  `desc:"current evaluation mode, e.g., Train, Test, etc"`
	Testing    bool    `desc:"if true, the model is being run in a testing mode, so no weight changes or other associated computations are needed.  this flag should only affect learning-related behavior"`

	TimePerCyc float32 `def:"0.001" desc:"amount of time to increment per cycle"`
}

// Defaults sets default values
func (tm *Time) Defaults() {
	tm.TimePerCyc = 0.001
}

// Reset resets the counters all back to zero
func (tm *Time) Reset() {
	tm.Phase = 0
	tm.PlusPhase = false
	tm.PhaseCycle = 0
	tm.Cycle = 0
	tm.CycleTot = 0
	tm.Time = 0
	tm.Testing = false
	if tm.TimePerCyc == 0 {
		tm.Defaults()
	}
}

// NewState resets counters at start of new state (trial) of processing.
// Pass the evaluation model associated with this new state --
// if !Train then testing will be set to true.
func (tm *Time) NewState(mode string) {
	tm.Phase = 0
	tm.PlusPhase = false
	tm.PhaseCycle = 0
	tm.Cycle = 0
	tm.Mode = mode
	tm.Testing = mode != "Train"
}

// NewPhase resets PhaseCycle = 0 and sets the plus phase as specified
func (tm *Time) NewPhase(plusPhase bool) {
	tm.PhaseCycle = 0
	tm.PlusPhase = plusPhase
}

// CycleInc increments at the cycle level
func (tm *Time) CycleInc() {
	tm.PhaseCycle++
	tm.Cycle++
	tm.CycleTot++
	tm.Time += tm.TimePerCyc
}

//gosl: end axon

// NewTime returns a new Time struct with default parameters
func NewTime() *Time {
	tm := &Time{}
	tm.Defaults()
	return tm
}
