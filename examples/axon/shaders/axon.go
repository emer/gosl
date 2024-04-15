package main

import (
	"cogentcore.org/core/math32"
	"github.com/emer/gosl/v2/slbool"
	"github.com/emer/gosl/v2/slrand"
	"github.com/emer/gosl/v2/sltype"
)

//gosl: hlsl axon
// #include "fastexp.hlsl"
//gosl: end axon

const (
	MaxFloat32 float32 = 3.402823466e+38
	MinFloat32 float32 = 1.175494351e-38
)

// F32 represents a min / max range for float32 values.
// Supports clipping, renormalizing, etc
type F32 struct {
	Min float32
	Max float32

	pad, pad1 float32
}

// IsValid returns true if Min <= Max
func (mr *F32) IsValid() bool {
	return mr.Min <= mr.Max
}

// InRange tests whether value is within the range (>= Min and <= Max)
func (mr *F32) InRange(val float32) bool {
	return ((val >= mr.Min) && (val <= mr.Max))
}

// IsLow tests whether value is lower than the minimum
func (mr *F32) IsLow(val float32) bool {
	return (val < mr.Min)
}

// IsHigh tests whether value is higher than the maximum
func (mr *F32) IsHigh(val float32) bool {
	return (val > mr.Min)
}

// SetInfinity sets the Min to +MaxFloat, Max to -MaxFloat -- suitable for
// iteratively calling Fit*InRange
func (mr *F32) SetInfinity() {
	mr.Min = MaxFloat32
	mr.Max = -MaxFloat32
}

// Range returns Max - Min
func (mr *F32) Range() float32 {
	return mr.Max - mr.Min
}

// Scale returns 1 / Range -- if Range = 0 then returns 0
func (mr *F32) Scale() float32 {
	var r float32
	r = mr.Range()
	if r != 0 {
		return 1.0 / r
	}
	return 0
}

// Midpoint returns point halfway between Min and Max
func (mr *F32) Midpoint() float32 {
	return 0.5 * (mr.Max + mr.Min)
}

// NormVal normalizes value to 0-1 unit range relative to current Min / Max range
// Clips the value within Min-Max range first.
func (mr *F32) NormValue(val float32) float32 {
	return (mr.ClipValue(val) - mr.Min) * mr.Scale()
}

// ProjVal projects a 0-1 normalized unit value into current Min / Max range (inverse of NormVal)
func (mr *F32) ProjValue(val float32) float32 {
	return mr.Min + (val * mr.Range())
}

// ClipVal clips given value within Min / Max range
// Note: a NaN will remain as a NaN
func (mr *F32) ClipValue(val float32) float32 {
	if val < mr.Min {
		return mr.Min
	}
	if val > mr.Max {
		return mr.Max
	}
	return val
}

// ClipNormVal clips then normalizes given value within 0-1
// Note: a NaN will remain as a NaN
func (mr *F32) ClipNormValue(val float32) float32 {
	if val < mr.Min {
		return 0
	}
	if val > mr.Max {
		return 1
	}
	return mr.NormValue(val)
}

// FitValInRange adjusts our Min, Max to fit given value within Min, Max range
// returns true if we had to adjust to fit.
func (mr *F32) FitValInRange(val float32) bool {
	var adj bool
	adj = false
	if val < mr.Min {
		mr.Min = val
		adj = true
	}
	if val > mr.Max {
		mr.Max = val
		adj = true
	}
	return adj
}

// Set sets the min and max values
func (mr *F32) Set(min, max float32) {
	mr.Min = min
	mr.Max = max
}

// Chans are ion channels used in computing point-neuron activation function
type Chans struct {

	// excitatory sodium (Na) AMPA channels activated by synaptic glutamate
	E float32

	// constant leak (potassium, K+) channels -- determines resting potential (typically higher than resting potential of K)
	L float32

	// inhibitory chloride (Cl-) channels activated by synaptic GABA
	I float32

	// gated / active potassium channels -- typically hyperpolarizing relative to leak / rest
	K float32
}

// VToBio returns biological mV voltage from normalized 0-1 voltage
// where 0 = -100mV and 1 = 0mV
func VToBio(vm float32) float32 {
	return vm*100 - 100
}

// VFromBio returns normalized 0-1 voltage from biological mV voltage
// where 0 = -100mV and 1 = 0mV
func VFromBio(vm float32) float32 {
	return (vm + 100) / 100
}

// AKsParams provides a highly simplified stateless A-type K channel
// that only has the voltage-gated activation (M) dynamic with a cutoff
// that ends up capturing a close approximation to the much more complex AK function.
// This is voltage gated with maximal activation around -37 mV.
// It is particularly important for counteracting the excitatory effects of
// voltage gated calcium channels which can otherwise drive runaway excitatory currents.
type AKsParams struct {

	// strength of AK current
	Gbar float32 `default:"2,0.1,0.01"`

	// H factor as a constant multiplier on overall M factor result -- rescales M to level consistent with H being present at full strength
	Hf float32 `default:"0.076"`

	// multiplier for M -- determines slope of function
	Mf float32 `default:"0.075"`

	// voltage offset in biological units for M function
	Voff float32 `default:"2"`
	Vmax float32 `def:-37" desc:"voltage level of maximum channel opening -- stays flat above that"`

	pad, pad1, pad2 float32
}

// Defaults sets the parameters for distal dendrites
func (ap *AKsParams) Defaults() {
	ap.Gbar = 0.1
	ap.Hf = 0.076
	ap.Mf = 0.075
	ap.Voff = 2
	ap.Vmax = -37
}

func (ap *AKsParams) Update() {
}

// MFromV returns the M gate function from vbio
func (ap *AKsParams) MFromV(vbio float32) float32 {
	if vbio > ap.Vmax {
		vbio = ap.Vmax
	}
	return ap.Hf / (1.0 + math32.FastExp(-ap.Mf*(vbio+ap.Voff)))
}

// MFromVnorm returns the M gate function from vnorm
func (ap *AKsParams) MFromVnorm(v float32) float32 {
	return ap.MFromV(VToBio(v))
}

// Gak returns the conductance as a function of normalized Vm
// GBar * MFromVnorm(v)
func (ap *AKsParams) Gak(v float32) float32 {
	return ap.Gbar * ap.MFromVnorm(v)
}

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

// KNaParams implements sodium (Na) gated potassium (K) currents
// that drive adaptation (accommodation) in neural firing.
// As neurons spike, driving an influx of Na, this activates
// the K channels, which, like leak channels, pull the membrane
// potential back down toward rest (or even below).
type KNaParams struct {

	// if On, use this component of K-Na adaptation
	On slbool.Bool

	// Rise rate of fast time-scale adaptation as function of Na concentration due to spiking -- directly multiplies -- 1/rise = tau for rise rate
	Rise float32

	// Maximum potential conductance of fast K channels -- divide nA biological value by 10 for the normalized units here
	Max float32

	// time constant in cycles for decay of adaptation, which should be milliseconds typically (tau is roughly how long it takes for value to change significantly -- 1.4x the half-life)
	Tau float32

	// 1/Tau rate constant
	Dt float32 `view:"-"`

	pad, pad1, pad2 float32
}

func (ka *KNaParams) Defaults() {
	ka.On = slbool.True
	ka.Rise = 0.01
	ka.Max = 0.1
	ka.Tau = 100
	ka.Update()
}

func (ka *KNaParams) Update() {
	ka.Dt = 1 / ka.Tau
}

// GcFromSpike updates the KNa conductance based on spike or not
func (ka *KNaParams) GcFromSpike(gKNa *float32, spike bool) {
	if slbool.IsTrue(ka.On) {
		if spike {
			*gKNa += ka.Rise * (ka.Max - *gKNa)
		} else {
			*gKNa -= ka.Dt * *gKNa
		}
	} else {
		*gKNa = 0
	}
}

// KNaMedSlow describes sodium-gated potassium channel adaptation mechanism.
// Evidence supports 2 different time constants:
// Slick (medium) and Slack (slow)
type KNaMedSlow struct {

	// if On, apply K-Na adaptation
	On slbool.Bool

	pad, pad1, pad2 float32

	// medium time-scale adaptation
	Med KNaParams `view:"inline"`

	// slow time-scale adaptation
	Slow KNaParams `view:"inline"`
}

func (ka *KNaMedSlow) Defaults() {
	ka.Med.Defaults()
	ka.Slow.Defaults()
	ka.Med.Tau = 200
	ka.Med.Rise = 0.02
	ka.Med.Max = 0.2
	ka.Slow.Tau = 1000
	ka.Slow.Rise = 0.001
	ka.Slow.Max = 0.2
	ka.Update()
}

func (ka *KNaMedSlow) Update() {
	ka.Med.Update()
	ka.Slow.Update()
}

// GcFromSpike updates med, slow time scales of KNa adaptation from spiking
func (ka *KNaMedSlow) GcFromSpike(gKNaM, gKNaS *float32, spike bool) {
	ka.Med.GcFromSpike(gKNaM, spike)
	ka.Slow.GcFromSpike(gKNaS, spike)
}

// MahpParams implements an M-type medium afterhyperpolarizing (mAHP) channel,
// where m also stands for muscarinic due to the ACh inactivation of this channel.
// It has a slow activation and deactivation time constant, and opens at a lowish
// membrane potential.
// There is one gating variable n updated over time with a tau that is also voltage dependent.
// The infinite-time value of n is voltage dependent according to a logistic function
// of the membrane potential, centered at Voff with slope Vslope.
type MahpParams struct {

	// strength of mAHP current
	Gbar float32

	// voltage offset (threshold) in biological units for infinite time N gating function -- where the gate is at 50% strength
	Voff float32 `default:"-30"`

	// slope of the arget (infinite time) gating function
	Vslope float32 `default:"9"`

	// maximum slow rate time constant in msec for activation / deactivation.  The effective Tau is much slower -- 1/20th in original temp, and 1/60th in standard 37 C temp
	TauMax float32 `default:"1000"`

	// temperature adjustment factor: assume temp = 37 C, whereas original units were at 23 C
	Tadj float32 `view:"-" edit:"-"`

	// 1/Tau
	DtMax     float32 `view:"-" edit:"-"`
	pad, pad1 float32
}

// Defaults sets the parameters
func (mp *MahpParams) Defaults() {
	mp.Gbar = 0.02
	mp.Voff = -30
	mp.Vslope = 9
	mp.TauMax = 1000
	mp.Tadj = math32.Pow(2.3, (37.0-23.0)/10.0) // 3.2 basically
	mp.Update()
}

func (mp *MahpParams) Update() {
	mp.DtMax = 1.0 / mp.TauMax
}

// EFun handles singularities in an elegant way -- from Mainen impl
func (mp *MahpParams) EFun(z float32) float32 {
	if math32.Abs(z) < 1.0e-4 {
		return 1.0 - 0.5*z
	}
	return z / (math32.FastExp(z) - 1.0)
}

// NinfTauFromV returns the target infinite-time N gate value and
// voltage-dependent time constant tau, from vbio
func (mp *MahpParams) NinfTauFromV(vbio float32, ninf, tau *float32) {
	var vo, a, b float32
	vo = vbio - mp.Voff

	// logical functions, but have signularity at Voff (vo = 0)
	// a := mp.DtMax * vo / (1.0 - math32.FastExp(-vo/mp.Vslope))
	// b := -mp.DtMax * vo / (1.0 - math32.FastExp(vo/mp.Vslope))

	a = mp.DtMax * mp.Vslope * mp.EFun(-vo/mp.Vslope)
	b = mp.DtMax * mp.Vslope * mp.EFun(vo/mp.Vslope)
	*tau = 1.0 / (a + b)
	*ninf = a * *tau // a / (a+b)
	*tau /= mp.Tadj  // correct right away..
}

// NinfTauFromVnorm returns the target infinite-time N gate value and
// voltage-dependent time constant tau, from normalized vm
func (mp *MahpParams) NinfTauFromVnorm(v float32, ninf, tau *float32) {
	mp.NinfTauFromV(VToBio(v), ninf, tau)
}

// DNFromV returns the change in gating factor N based on normalized Vm
func (mp *MahpParams) DNFromV(v, n float32) float32 {
	var ninf, tau float32
	mp.NinfTauFromVnorm(v, &ninf, &tau)
	// dt := 1.0 - math32.FastExp(-mp.Tadj/tau) // Mainen comments out this form; Poirazi uses
	// dt := mp.Tadj / tau // simple linear fix
	return (ninf - n) / tau
}

// GmAHP returns the conductance as a function of n
func (mp *MahpParams) GmAHP(n float32) float32 {
	return mp.Tadj * mp.Gbar * n
}

// NMDAParams control the NMDA dynamics, based on Jahr & Stevens (1990) equations
// which are widely used in models, from Brunel & Wang (2001) to Sanders et al. (2013).
// The overall conductance is a function of a voltage-dependent postsynaptic factor based
// on Mg ion blockage, and presynaptic Glu-based opening, which in a simple model just
// increments
type NMDAParams struct {

	// overall multiplier for strength of NMDA current -- multiplies GnmdaSyn to get net conductance.  0.15 standard for SnmdaDeplete = false, 1.4 when on.
	Gbar float32 `default:"0,0.15,0.25,0.3,1.4"`

	// decay time constant for NMDA channel activation  -- rise time is 2 msec and not worth extra effort for biexponential.  30 fits the Urakubo et al (2008) model with ITau = 100, but 100 works better in practice is small networks so far.
	Tau float32 `default:"30,50,100,200,300"`

	// decay time constant for NMDA channel inhibition, which captures the Urakubo et al (2008) allosteric dynamics (100 fits their model well) -- set to 1 to eliminate that mechanism.
	ITau float32 `default:"1,100"`

	// magnesium ion concentration: Brunel & Wang (2001) and Sanders et al (2013) use 1 mM, based on Jahr & Stevens (1990). Urakubo et al (2008) use 1.5 mM. 1.4 with Voff = 5 works best so far in large models, 1.2, Voff = 0 best in smaller nets.
	MgC float32 `default:"1:1.5"`

	// offset in membrane potential in biological units for voltage-dependent functions.  5 corresponds to the -65 mV rest, -45 threshold of the Urakubo et al (2008) model.  0 is best in small models
	Voff float32 `default:"0,5"`

	// rate = 1 / tau
	Dt float32 `view:"-" json:"-" xml:"-"`

	// rate = 1 / tau
	IDt float32 `view:"-" json:"-" xml:"-"`

	// MgFact = MgC / 3.57
	MgFact float32 `view:"-" json:"-" xml:"-"`
}

func (np *NMDAParams) Defaults() {
	np.Gbar = 0.15
	np.Tau = 100
	np.ITau = 1 // off by default, as it doesn't work in actual axon models..
	np.MgC = 1.4
	np.Voff = 5
	np.Update()
}

func (np *NMDAParams) Update() {
	np.Dt = 1 / np.Tau
	np.IDt = 1 / np.ITau
	np.MgFact = np.MgC / 3.57
}

// MgGFromVbio returns the NMDA conductance as a function of biological membrane potential
// based on Mg ion blocking
func (np *NMDAParams) MgGFromVbio(vbio float32) float32 {
	vbio += np.Voff
	if vbio >= 0 {
		return 0
	}
	return 1.0 / (1.0 + np.MgFact*math32.FastExp(-0.062*vbio))
}

// MgGFromV returns the NMDA conductance as a function of normalized membrane potential
// based on Mg ion blocking
func (np *NMDAParams) MgGFromV(v float32) float32 {
	return np.MgGFromVbio(VToBio(v))
}

// CaFromVbio returns the calcium current factor as a function of biological membrane
// potential -- this factor is needed for computing the calcium current * MgGFromV.
// This is the same function used in VGCC for their conductance factor.
func (np *NMDAParams) CaFromVbio(vbio float32) float32 {
	vbio += np.Voff
	if vbio > -0.1 && vbio < 0.1 {
		return 1.0 / (0.0756 + 0.5*vbio)
	}
	return -vbio / (1.0 - math32.FastExp(0.0756*vbio))
}

// CaFromV returns the calcium current factor as a function of normalized membrane
// potential -- this factor is needed for computing the calcium current * MgGFromV
func (np *NMDAParams) CaFromV(v float32) float32 {
	return np.CaFromVbio(VToBio(v))
}

// NMDASyn returns the updated synaptic NMDA Glu binding
// based on new raw spike-driven Glu binding.
func (np *NMDAParams) NMDASyn(nmda, raw float32) float32 {
	return nmda + raw - np.Dt*nmda
}

// Gnmda returns the NMDA net conductance from nmda Glu binding and Vm
// including the GBar factor
func (np *NMDAParams) Gnmda(nmda, vm float32) float32 {
	return np.Gbar * np.MgGFromV(vm) * nmda
}

// SnmdaFromSpike updates sender-based NMDA channel opening based on neural spiking
// using the inhibition and decay factors.  These dynamics closely match the
// Urakubo et al (2008) allosteric NMDA receptor behavior, with ITau = 100, Tau = 30
func (np *NMDAParams) SnmdaFromSpike(spike float32, snmdaO, snmdaI *float32) {
	if spike > 0 {
		var inh float32
		inh = (1 - *snmdaI)
		*snmdaO += inh * (1 - *snmdaO)
		*snmdaI += inh
	} else {
		*snmdaO -= np.Dt * *snmdaO
		*snmdaI -= np.IDt * *snmdaI
	}
}

// SahpParams implements a slow afterhyperpolarizing (sAHP) channel,
// It has a slowly accumulating calcium value, aggregated at the
// theta cycle level, that then drives the logistic gating function,
// so that it only activates after a significant accumulation.
// After which point it decays.
// For the theta-cycle updating, the normal m-type tau is all within
// the scope of a single theta cycle, so we just omit the time integration
// of the n gating value, but tau is computed in any case.
type SahpParams struct {

	// strength of sAHP current
	Gbar float32 `default:"0.05,0.1"`

	// time constant for integrating Ca across theta cycles
	CaTau float32 `default:"5,10"`

	// integrated Ca offset (threshold) for infinite time N gating function -- where the gate is at 50% strength
	Off float32 `default:"0.8"`

	// slope of the infinite time logistic gating function
	Slope float32 `default:"0.02"`

	// maximum slow rate time constant in msec for activation / deactivation.  The effective Tau is much slower -- 1/20th in original temp, and 1/60th in standard 37 C temp
	TauMax float32 `default:"1"`

	// 1/Tau
	CaDt float32 `view:"-" edit:"-"`

	// 1/Tau
	DtMax float32 `view:"-" edit:"-"`
	pad   float32
}

// Defaults sets the parameters
func (mp *SahpParams) Defaults() {
	mp.Gbar = 0.05
	mp.CaTau = 5
	mp.Off = 0.8
	mp.Slope = 0.02
	mp.TauMax = 1
	mp.Update()
}

func (mp *SahpParams) Update() {
	mp.DtMax = 1.0 / mp.TauMax
	mp.CaDt = 1.0 / mp.CaTau
}

// EFun handles singularities in an elegant way -- from Mainen impl
func (mp *SahpParams) EFun(z float32) float32 {
	if math32.Abs(z) < 1.0e-4 {
		return 1.0 - 0.5*z
	}
	return z / (math32.FastExp(z) - 1.0)
}

// NinfTauFromCa returns the target infinite-time N gate value and
// time constant tau, from integrated Ca value
func (mp *SahpParams) NinfTauFromCa(ca float32, ninf, tau *float32) {
	var co, a, b float32
	co = ca - mp.Off

	// logical functions, but have signularity at Voff (vo = 0)
	// a := mp.DtMax * vo / (1.0 - math32.FastExp(-vo/mp.Vslope))
	// b := -mp.DtMax * vo / (1.0 - math32.FastExp(vo/mp.Vslope))

	a = mp.DtMax * mp.Slope * mp.EFun(-co/mp.Slope)
	b = mp.DtMax * mp.Slope * mp.EFun(co/mp.Slope)
	*tau = 1.0 / (a + b)
	*ninf = a * *tau // a / (a+b)
}

// CaInt returns the updated time-integrated Ca value from current value and current Ca
func (mp *SahpParams) CaInt(caInt, ca float32) float32 {
	caInt += mp.CaDt * (ca - caInt)
	return caInt
}

// DNFromCa returns the change in gating factor N based on integrated Ca
// Omit this and just use ninf directly for theta-cycle updating.
func (mp *SahpParams) DNFromV(ca, n float32) float32 {
	var ninf, tau float32
	mp.NinfTauFromCa(ca, &ninf, &tau)
	return (ninf - n) / tau
}

// GsAHP returns the conductance as a function of n
func (mp *SahpParams) GsAHP(n float32) float32 {
	return mp.Gbar * n
}

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
	return -vbio / (1.0 - math32.FastExp(0.0756*vbio))
}

// MFromV returns the M gate function from vbio (not normalized, must not exceed 0)
func (np *VGCCParams) MFromV(vbio float32) float32 {
	if vbio < -60 {
		return 0
	}
	if vbio > -10 {
		return 1
	}
	return 1.0 / (1.0 + math32.FastExp(-(vbio + 37)))
}

// HFromV returns the H gate function from vbio (not normalized, must not exceed 0)
func (np *VGCCParams) HFromV(vbio float32) float32 {
	if vbio < -50 {
		return 1
	}
	if vbio > -10 {
		return 0
	}
	return 1.0 / (1.0 + math32.FastExp((vbio+41)*2))
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

//gosl: hlsl axon
// #include "slrand.hlsl"
//gosl: end axon

// axon.Time contains all the timing state and parameter information for running a model.
// Can also include other relevant state context, e.g., Testing vs. Training modes.
type Time struct {

	// phase counter: typicaly 0-1 for minus-plus but can be more phases for other algorithms
	Phase int32

	// true if this is the plus phase, when the outcome / bursting is occurring, driving positive learning -- else minus phase
	PlusPhase slbool.Bool

	// cycle within current phase -- minus or plus
	PhaseCycle int32

	// cycle counter: number of iterations of activation updating (settling) on the current state -- this counts time sequentially until reset with NewState
	Cycle int32

	// total cycle count -- this increments continuously from whenever it was last reset -- typically this is number of milliseconds in simulation time
	CycleTot int32

	// accumulated amount of time the network has been running, in simulation-time (not real world time), in seconds
	Time float32

	// if true, the model is being run in a testing mode, so no weight changes or other associated computations are needed.  this flag should only affect learning-related behavior
	Testing slbool.Bool

	// amount of time to increment per cycle
	TimePerCyc float32 `default:"0.001"`

	// random counter
	RandCtr slrand.Counter
}

// Defaults sets default values
func (tm *Time) Defaults() {
	tm.TimePerCyc = 0.001
}

// Reset resets the counters all back to zero
func (tm *Time) Reset() {
	tm.Phase = 0
	tm.PlusPhase = slbool.False
	tm.PhaseCycle = 0
	tm.Cycle = 0
	tm.CycleTot = 0
	tm.Time = 0
	tm.Testing = slbool.False
	if tm.TimePerCyc == 0 {
		tm.TimePerCyc = 0.001
	}
	tm.RandCtr.Reset()
}

// NewState resets counters at start of new state (trial) of processing.
// Pass the evaluation model associated with this new state --
// if !Train then testing will be set to true.
func (tm *Time) NewState() {
	tm.Phase = 0
	tm.PlusPhase = slbool.False
	tm.PhaseCycle = 0
	tm.Cycle = 0
	// tm.Testing = mode != "Train"
}

// NewPhase resets PhaseCycle = 0 and sets the plus phase as specified
func (tm *Time) NewPhase(plusPhase bool) {
	tm.PhaseCycle = 0
	tm.PlusPhase = slbool.FromBool(plusPhase)
}

// CycleInc increments at the cycle level
func (tm *Time) CycleInc() {
	tm.PhaseCycle++
	tm.Cycle++
	tm.CycleTot++
	tm.Time += tm.TimePerCyc
}

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

//////////////////////////////////////////////////////////////////////////////////////
//  SpikeParams

// SpikeParams contains spiking activation function params.
// Implements a basic thresholded Vm model, and optionally
// the AdEx adaptive exponential function (adapt is KNaAdapt)
type SpikeParams struct {

	// threshold value Theta (Q) for firing output activation (.5 is more accurate value based on AdEx biological parameters and normalization
	Thr float32 `default:"0.5"`

	// post-spiking membrane potential to reset to, produces refractory effect if lower than VmInit -- 0.3 is apropriate biologically-based value for AdEx (Brette & Gurstner, 2005) parameters.  See also RTau
	VmR float32 `default:"0.3"`

	// post-spiking explicit refractory period, in cycles -- prevents Vm updating for this number of cycles post firing -- Vm is reduced in exponential steps over this period according to RTau, being fixed at Tr to VmR exactly
	Tr int32 `min:"1" default:"3"`

	// time constant for decaying Vm down to VmR -- at end of Tr it is set to VmR exactly -- this provides a more realistic shape of the post-spiking Vm which is only relevant for more realistic channels that key off of Vm -- does not otherwise affect standard computation
	RTau float32 `default:"1.6667"`

	// if true, turn on exponential excitatory current that drives Vm rapidly upward for spiking as it gets past its nominal firing threshold (Thr) -- nicely captures the Hodgkin Huxley dynamics of Na and K channels -- uses Brette & Gurstner 2005 AdEx formulation
	Exp slbool.Bool `default:"true"`

	// slope in Vm (2 mV = .02 in normalized units) for extra exponential excitatory current that drives Vm rapidly upward for spiking as it gets past its nominal firing threshold (Thr) -- nicely captures the Hodgkin Huxley dynamics of Na and K channels -- uses Brette & Gurstner 2005 AdEx formulation
	ExpSlope float32 `default:"0.02"`

	// membrane potential threshold for actually triggering a spike when using the exponential mechanism
	ExpThr float32 `default:"0.9"`

	// for translating spiking interval (rate) into rate-code activation equivalent, what is the maximum firing rate associated with a maximum activation value of 1
	MaxHz float32 `default:"180" min:"1"`

	// constant for integrating the spiking interval in estimating spiking rate
	ISITau float32 `default:"5" min:"1"`

	// rate = 1 / tau
	ISIDt float32 `view:"-"`

	// rate = 1 / tau
	RDt float32 `view:"-"`

	pad float32
}

func (sk *SpikeParams) Defaults() {
	sk.Thr = 0.5
	sk.VmR = 0.3
	sk.Tr = 3
	sk.RTau = 1.6667
	sk.Exp.SetBool(true)
	sk.ExpSlope = 0.02
	sk.ExpThr = 0.9
	sk.MaxHz = 180
	sk.ISITau = 5
	sk.Update()
}

func (sk *SpikeParams) Update() {
	if sk.Tr <= 0 {
		sk.Tr = 1 // hard min
	}
	sk.ISIDt = 1 / sk.ISITau
	sk.RDt = 1 / sk.RTau
}

// ActToISI compute spiking interval from a given rate-coded activation,
// based on time increment (.001 = 1msec default), Act.Dt.Integ
func (sk *SpikeParams) ActToISI(act, timeInc, integ float32) float32 {
	if act == 0 {
		return 0
	}
	return (1 / (timeInc * integ * act * sk.MaxHz))
}

// ActFromISI computes rate-code activation from estimated spiking interval
func (sk *SpikeParams) ActFromISI(isi, timeInc, integ float32) float32 {
	if isi <= 0 {
		return 0
	}
	maxInt := 1.0 / (timeInc * integ * sk.MaxHz) // interval at max hz..
	return maxInt / isi                          // normalized
}

// AvgFromISI updates spiking ISI from current isi interval value
func (sk *SpikeParams) AvgFromISI(avg *float32, isi float32) {
	if *avg <= 0 {
		*avg = isi
	} else if isi < 0.8**avg {
		*avg = isi // if significantly less than we take that
	} else { // integrate on slower
		*avg += sk.ISIDt * (isi - *avg) // running avg updt
	}
}

//////////////////////////////////////////////////////////////////////////////////////
//  DendParams

// DendParams are the parameters for updating dendrite-specific dynamics
type DendParams struct {

	// dendrite-specific strength multiplier of the exponential spiking drive on Vm -- e.g., .5 makes it half as strong as at the soma (which uses Gbar.L as a strength multiplier per the AdEx standard model)
	GbarExp float32 `default:"0.2,0.5"`

	// dendrite-specific conductance of Kdr delayed rectifier currents, used to reset membrane potential for dendrite -- applied for Tr msec
	GbarR float32 `default:"3,6"`

	// SST+ somatostatin positive slow spiking inhibition level specifically affecting dendritic Vm (VmDend) -- this is important for countering a positive feedback loop from NMDA getting stronger over the course of learning -- also typically requires SubMean = 1 for TrgAvgAct and learning to fully counter this feedback loop.
	SSGi float32 `default:"0,2"`

	pad float32
}

func (dp *DendParams) Defaults() {
	dp.SSGi = 2
	dp.GbarExp = 0.2
	dp.GbarR = 3
}

func (dp *DendParams) Update() {
}

//////////////////////////////////////////////////////////////////////////////////////
//  ActInitParams

// ActInitParams are initial values for key network state variables.
// Initialized in InitActs called by InitWts, and provides target values for DecayState.
type ActInitParams struct {

	// initial membrane potential -- see Erev.L for the resting potential (typically .3)
	Vm float32 `default:"0.3"`

	// initial activation value -- typically 0
	Act float32 `default:"0"`

	// baseline level of excitatory conductance (net input) -- Ge is initialized to this value, and it is added in as a constant background level of excitatory input -- captures all the other inputs not represented in the model, and intrinsic excitability, etc
	Ge float32 `default:"0"`

	// baseline level of inhibitory conductance (net input) -- Gi is initialized to this value, and it is added in as a constant background level of inhibitory input -- captures all the other inputs not represented in the model
	Gi float32 `default:"0"`

	// variance (sigma) of gaussian distribution around baseline Ge values, per unit, to establish variability in intrinsic excitability.  value never goes < 0
	GeVar float32 `default:"0"`

	// variance (sigma) of gaussian distribution around baseline Gi values, per unit, to establish variability in intrinsic excitability.  value never goes < 0
	GiVar float32 `default:"0"`

	pad, pad1 float32
}

func (ai *ActInitParams) Update() {
}

func (ai *ActInitParams) Defaults() {
	ai.Vm = 0.3
	ai.Act = 0
	ai.Ge = 0
	ai.Gi = 0
	ai.GeVar = 0
	ai.GiVar = 0
}

//////////////////////////////////////////////////////////////////////////////////////
//  DecayParams

// DecayParams control the decay of activation state in the DecayState function
// called in NewState when a new state is to be processed.
type DecayParams struct {

	// proportion to decay most activation state variables toward initial values at start of every ThetaCycle (except those controlled separately below) -- if 1 it is effectively equivalent to full clear, resetting other derived values.  ISI is reset every AlphaCycle to get a fresh sample of activations (doesn't affect direct computation -- only readout).
	Act float32 `default:"0,0.2,0.5,1" max:"1" min:"0"`

	// proportion to decay long-lasting conductances, NMDA and GABA, and also the dendritic membrane potential -- when using random stimulus order, it is important to decay this significantly to allow a fresh start -- but set Act to 0 to enable ongoing activity to keep neurons in their sensitive regime.
	Glong float32 `default:"0,0.6" max:"1" min:"0"`

	// decay of afterhyperpolarization currents, including mAHP, sAHP, and KNa -- has a separate decay because often useful to have this not decay at all even if decay is on.
	AHP float32 `default:"0" max:"1" min:"0"`

	pad float32
}

func (ai *DecayParams) Update() {
}

func (ai *DecayParams) Defaults() {
	ai.Act = 0.2
	ai.Glong = 0.6
	ai.AHP = 0
}

//////////////////////////////////////////////////////////////////////////////////////
//  DtParams

// DtParams are time and rate constants for temporal derivatives in Axon (Vm, G)
type DtParams struct {

	// overall rate constant for numerical integration, for all equations at the unit level -- all time constants are specified in millisecond units, with one cycle = 1 msec -- if you instead want to make one cycle = 2 msec, you can do this globally by setting this integ value to 2 (etc).  However, stability issues will likely arise if you go too high.  For improved numerical stability, you may even need to reduce this value to 0.5 or possibly even lower (typically however this is not necessary).  MUST also coordinate this with network.time_inc variable to ensure that global network.time reflects simulated time accurately
	Integ float32 `default:"1,0.5" min:"0"`

	// membrane potential time constant in cycles, which should be milliseconds typically (tau is roughly how long it takes for value to change significantly -- 1.4x the half-life) -- reflects the capacitance of the neuron in principle -- biological default for AdEx spiking model C = 281 pF = 2.81 normalized
	VmTau float32 `default:"2.81" min:"1"`

	// dendritic membrane potential time constant in cycles, which should be milliseconds typically (tau is roughly how long it takes for value to change significantly -- 1.4x the half-life) -- reflects the capacitance of the neuron in principle -- biological default for AdEx spiking model C = 281 pF = 2.81 normalized
	VmDendTau float32 `default:"5" min:"1"`

	// number of integration steps to take in computing new Vm value -- this is the one computation that can be most numerically unstable so taking multiple steps with proportionally smaller dt is beneficial
	VmSteps int32 `default:"2" min:"1"`

	// time constant for decay of excitatory AMPA receptor conductance.
	GeTau float32 `default:"5" min:"1"`

	// time constant for decay of inhibitory GABAa receptor conductance.
	GiTau float32 `default:"7" min:"1"`

	// time constant for integrating values over timescale of an individual input state (e.g., roughly 200 msec -- theta cycle), used in computing ActInt, and for GeM from Ge -- this is used for scoring performance, not for learning, in cycles, which should be milliseconds typically (tau is roughly how long it takes for value to change significantly -- 1.4x the half-life),
	IntTau float32 `default:"40" min:"1"`

	// time constant for integrating slower long-time-scale averages, such as nrn.ActAvg, Pool.ActsMAvg, ActsPAvg -- computed in NewState when a new input state is present (i.e., not msec but in units of a theta cycle) (tau is roughly how long it takes for value to change significantly) -- set lower for smaller models
	LongAvgTau float32 `default:"20" min:"1"`

	// cycle to start updating the SpkMaxCa, SpkMax values within a theta cycle -- early cycles often reflect prior state
	MaxCycStart int32 `default:"50" min:"0"`

	// nominal rate = Integ / tau
	VmDt float32 `view:"-" json:"-" xml:"-"`

	// nominal rate = Integ / tau
	VmDendDt float32 `view:"-" json:"-" xml:"-"`

	// 1 / VmSteps
	DtStep float32 `view:"-" json:"-" xml:"-"`

	// rate = Integ / tau
	GeDt float32 `view:"-" json:"-" xml:"-"`

	// rate = Integ / tau
	GiDt float32 `view:"-" json:"-" xml:"-"`

	// rate = Integ / tau
	IntDt float32 `view:"-" json:"-" xml:"-"`

	// rate = 1 / tau
	LongAvgDt float32 `view:"-" json:"-" xml:"-"`
}

func (dp *DtParams) Update() {
	if dp.VmSteps < 1 {
		dp.VmSteps = 1
	}
	dp.VmDt = dp.Integ / dp.VmTau
	dp.VmDendDt = dp.Integ / dp.VmDendTau
	dp.DtStep = 1 / float32(dp.VmSteps)
	dp.GeDt = dp.Integ / dp.GeTau
	dp.GiDt = dp.Integ / dp.GiTau
	dp.IntDt = dp.Integ / dp.IntTau
	dp.LongAvgDt = 1 / dp.LongAvgTau
}

func (dp *DtParams) Defaults() {
	dp.Integ = 1
	dp.VmTau = 2.81
	dp.VmDendTau = 5
	dp.VmSteps = 2
	dp.GeTau = 5
	dp.GiTau = 7
	dp.IntTau = 40
	dp.LongAvgTau = 20
	dp.MaxCycStart = 50
	dp.Update()
}

// GeSynFromRaw integrates a synaptic conductance from raw spiking using GeTau
func (dp *DtParams) GeSynFromRaw(geSyn, geRaw float32) float32 {
	return geSyn + geRaw - dp.GeDt*geSyn
}

// GeSynFromRawSteady returns the steady-state GeSyn that would result from
// receiving a steady increment of GeRaw every time step = raw * GeTau.
// dSyn = Raw - dt*Syn; solve for dSyn = 0 to get steady state:
// dt*Syn = Raw; Syn = Raw / dt = Raw * Tau
func (dp *DtParams) GeSynFromRawSteady(geRaw float32) float32 {
	return geRaw * dp.GeTau
}

// GiSynFromRaw integrates a synaptic conductance from raw spiking using GiTau
func (dp *DtParams) GiSynFromRaw(giSyn, giRaw float32) float32 {
	return giSyn + giRaw - dp.GiDt*giSyn
}

// GiSynFromRawSteady returns the steady-state GiSyn that would result from
// receiving a steady increment of GiRaw every time step = raw * GiTau.
// dSyn = Raw - dt*Syn; solve for dSyn = 0 to get steady state:
// dt*Syn = Raw; Syn = Raw / dt = Raw * Tau
func (dp *DtParams) GiSynFromRawSteady(giRaw float32) float32 {
	return giRaw * dp.GiTau
}

// AvgVarUpdate updates the average and variance from current value, using LongAvgDt
func (dp *DtParams) AvgVarUpdate(avg, vr *float32, val float32) {
	if *avg == 0 { // first time -- set
		*avg = val
		*vr = 0
	} else {
		del := val - *avg
		incr := dp.LongAvgDt * del
		*avg += incr
		// following is magic exponentially-weighted incremental variance formula
		// derived by Finch, 2009: Incremental calculation of weighted mean and variance
		if *vr == 0 {
			*vr = 2 * (1 - dp.LongAvgDt) * del * incr
		} else {
			*vr = (1 - dp.LongAvgDt) * (*vr + del*incr)
		}
	}
}

//////////////////////////////////////////////////////////////////////////////////////
//  Noise

// SpikeNoiseParams parameterizes background spiking activity impinging on the neuron,
// simulated using a poisson spiking process.
type SpikeNoiseParams struct {

	// add noise simulating background spiking levels
	On slbool.Bool

	// mean frequency of excitatory spikes -- typically 50Hz but multiple inputs increase rate -- poisson lambda parameter, also the variance
	GeHz float32 `default:"100"`

	// excitatory conductance per spike -- .001 has minimal impact, .01 can be strong, and .15 is needed to influence timing of clamped inputs
	Ge float32 `min:"0"`

	// mean frequency of inhibitory spikes -- typically 100Hz fast spiking but multiple inputs increase rate -- poisson lambda parameter, also the variance
	GiHz float32 `default:"200"`

	// excitatory conductance per spike -- .001 has minimal impact, .01 can be strong, and .15 is needed to influence timing of clamped inputs
	Gi float32 `min:"0"`

	// Exp(-Interval) which is the threshold for GeNoiseP as it is updated
	GeExpInt float32 `view:"-" json:"-" xml:"-"`

	// Exp(-Interval) which is the threshold for GiNoiseP as it is updated
	GiExpInt float32 `view:"-" json:"-" xml:"-"`

	pad float32
}

func (an *SpikeNoiseParams) Update() {
	an.GeExpInt = math32.Exp(-1000.0 / an.GeHz)
	an.GiExpInt = math32.Exp(-1000.0 / an.GiHz)
}

func (an *SpikeNoiseParams) Defaults() {
	an.GeHz = 100
	an.Ge = 0.001
	an.GiHz = 200
	an.Gi = 0.001
	an.Update()
}

// PGe updates the GeNoiseP probability, multiplying a uniform random number [0-1]
// and returns Ge from spiking if a spike is triggered
func (an *SpikeNoiseParams) PGe(p *float32, ni int, randctr *sltype.Uint2) float32 {
	*p *= slrand.Float(randctr, uint32(ni))
	if *p <= an.GeExpInt {
		*p = 1
		return an.Ge
	}
	return 0
}

// PGi updates the GiNoiseP probability, multiplying a uniform random number [0-1]
// and returns Gi from spiking if a spike is triggered
func (an *SpikeNoiseParams) PGi(p *float32, ni int, randctr *sltype.Uint2) float32 {
	*p *= slrand.Float(randctr, uint32(ni))
	if *p <= an.GiExpInt {
		*p = 1
		return an.Gi
	}
	return 0
}

//////////////////////////////////////////////////////////////////////////////////////
//  ClampParams

// ClampParams specify how external inputs drive excitatory conductances
// (like a current clamp) -- either adds or overwrites existing conductances.
// Noise is added in either case.
type ClampParams struct {

	// amount of Ge driven for clamping -- generally use 0.8 for Target layers, 1.5 for Input layers
	Ge float32 `default:"0.8,1.5"`

	//
	Add slbool.Bool `default:"false" view:"add external conductance on top of any existing -- generally this is not a good idea for target layers (creates a main effect that learning can never match), but may be ok for input layers"`

	// threshold on neuron Act activity to count as active for computing error relative to target in PctErr method
	ErrThr float32 `default:"0.5"`

	pad float32
}

func (cp *ClampParams) Update() {
}

func (cp *ClampParams) Defaults() {
	cp.Ge = 0.8
	cp.ErrThr = 0.5
}

//////////////////////////////////////////////////////////////////////////////////////
//  AttnParams

// AttnParams determine how the Attn modulates Ge
type AttnParams struct {

	// is attentional modulation active?
	On slbool.Bool

	// minimum act multiplier if attention is 0
	Min float32

	pad, pad1 float32
}

func (at *AttnParams) Defaults() {
	at.On.SetBool(true)
	at.Min = 0.8
}

func (at *AttnParams) Update() {
}

// ModVal returns the attn-modulated value -- attn must be between 1-0
func (at *AttnParams) ModValue(val float32, attn float32) float32 {
	if val < 0 {
		val = 0
	}
	if at.On.IsFalse() {
		return val
	}
	return val * (at.Min + (1-at.Min)*attn)
}

//////////////////////////////////////////////////////////////////////////////////////
//  SynComParams

// SynComParams are synaptic communication parameters: delay and probability of failure
type SynComParams struct {

	// additional synaptic delay for inputs arriving at this projection -- IMPORTANT: if you change this, you must call InitWts() on Network!  Delay = 0 means a spike reaches receivers in the next Cycle, which is the minimum time.  Biologically, subtract 1 from synaptic delay values to set corresponding Delay value.
	Delay int32 `min:"0" default:"2"`

	// probability of synaptic transmission failure -- if > 0, then weights are turned off at random as a function of PFail (times 1-SWt if PFailSwt)
	PFail float32

	// if true, then probability of failure is inversely proportional to SWt structural / slow weight value (i.e., multiply PFail * (1-SWt)))
	PFailSWt slbool.Bool

	pad float32
}

func (sc *SynComParams) Defaults() {
	sc.Delay = 2
	sc.PFail = 0 // 0.5 works?
	sc.PFailSWt.SetBool(false)
}

func (sc *SynComParams) Update() {
}

// WtFailP returns probability of weight (synapse) failure given current SWt value
func (sc *SynComParams) WtFailP(swt float32) float32 {
	if sc.PFailSWt.IsFalse() {
		return sc.PFail
	}
	return sc.PFail * (1 - swt)
}

// axon.ActParams contains all the activation computation params and functions
// for basic Axon, at the neuron level .
// This is included in axon.Layer to drive the computation.
type ActParams struct {

	// Spiking function parameters
	Spike SpikeParams `view:"inline"`

	// dendrite-specific parameters
	Dend DendParams `view:"inline"`

	// initial values for key network state variables -- initialized in InitActs called by InitWts, and provides target values for DecayState
	Init ActInitParams `view:"inline"`

	// amount to decay between AlphaCycles, simulating passage of time and effects of saccades etc, especially important for environments with random temporal structure (e.g., most standard neural net training corpora)
	Decay DecayParams `view:"inline"`

	// time and rate constants for temporal derivatives / updating of activation state
	Dt DtParams `view:"inline"`

	// maximal conductances levels for channels
	Gbar Chans `view:"inline"`

	// reversal potentials for each channel
	Erev Chans `view:"inline"`

	// how external inputs drive neural activations
	Clamp ClampParams `view:"inline"`

	// how, where, when, and how much noise to add
	Noise SpikeNoiseParams `view:"inline"`

	// range for Vm membrane potential -- -- important to keep just at extreme range of reversal potentials to prevent numerical instability
	VmRange F32 `view:"inline"`

	// M-type medium time-scale afterhyperpolarization mAHP current -- this is the primary form of adaptation on the time scale of multiple sequences of spikes
	Mahp MahpParams `view:"inline"`

	// slow time-scale afterhyperpolarization sAHP current -- integrates SpkCaD at theta cycle intervals and produces a hard cutoff on sustained activity for any neuron
	Sahp SahpParams `view:"inline"`

	// sodium-gated potassium channel adaptation parameters -- activates a leak-like current as a function of neural activity (firing = Na influx) at two different time-scales (Slick = medium, Slack = slow)
	KNa KNaMedSlow `view:"inline"`

	// NMDA channel parameters used in computing Gnmda conductance for bistability, and postsynaptic calcium flux used in learning.  Note that Learn.Snmda has distinct parameters used in computing sending NMDA parameters used in learning.
	NMDA NMDAParams `view:"inline"`

	// GABA-B / GIRK channel parameters
	GABAB GABABParams `view:"inline"`

	// voltage gated calcium channels -- provide a key additional source of Ca for learning and positive-feedback loop upstate for active neurons
	VGCC VGCCParams `view:"inline"`

	// A-type potassium (K) channel that is particularly important for limiting the runaway excitation from VGCC channels
	AK AKsParams `view:"inline"`

	// Attentional modulation parameters: how Attn modulates Ge
	Attn AttnParams `view:"inline"`
}

func (ac *ActParams) Defaults() {
	ac.Spike.Defaults()
	ac.Dend.Defaults()
	ac.Init.Defaults()
	ac.Decay.Defaults()
	ac.Dt.Defaults()
	ac.Gbar.SetAll(1.0, 0.2, 1.0, 1.0) // E, L, I, K: gbar l = 0.2 > 0.1
	ac.Erev.SetAll(1.0, 0.3, 0.1, 0.1) // E, L, I, K: K = hyperpolarized -90mv
	ac.Clamp.Defaults()
	ac.Noise.Defaults()
	ac.VmRange.Set(0.1, 1.0)
	ac.Mahp.Defaults()
	ac.Mahp.Gbar = 0.02
	ac.Sahp.Defaults()
	ac.Sahp.Gbar = 0.05
	ac.Sahp.CaTau = 5
	ac.KNa.Defaults()
	ac.KNa.On = slbool.True
	ac.NMDA.Defaults()
	ac.NMDA.Gbar = 0.15 // .15 now -- was 0.3 best.
	ac.GABAB.Defaults()
	ac.VGCC.Defaults()
	ac.VGCC.Gbar = 0.02
	ac.VGCC.Ca = 25
	ac.AK.Defaults()
	ac.AK.Gbar = 0.1
	ac.Attn.Defaults()
	ac.Update()
}

// Update must be called after any changes to parameters
func (ac *ActParams) Update() {
	ac.Spike.Update()
	ac.Dend.Update()
	ac.Init.Update()
	ac.Decay.Update()
	ac.Dt.Update()
	ac.Clamp.Update()
	ac.Noise.Update()
	ac.Mahp.Update()
	ac.Sahp.Update()
	ac.KNa.Update()
	ac.NMDA.Update()
	ac.GABAB.Update()
	ac.VGCC.Update()
	ac.AK.Update()
	ac.Attn.Update()
}

///////////////////////////////////////////////////////////////////////
//  Init

// DecayState decays the activation state toward initial values
// in proportion to given decay parameter.  Special case values
// such as Glong and KNa are also decayed with their
// separately parameterized values.
// Called with ac.Decay.Act by Layer during NewState
func (ac *ActParams) DecayState(nrn *Neuron, decay, glong float32) {
	// always reset these -- otherwise get insanely large values that take forever to update
	nrn.ISI = -1
	nrn.ISIAvg = -1
	nrn.ActInt = ac.Init.Act // start fresh

	if decay > 0 { // no-op for most, but not all..
		nrn.Spike = 0
		nrn.Spiked = 0
		nrn.Act -= decay * (nrn.Act - ac.Init.Act)
		nrn.ActInt -= decay * (nrn.ActInt - ac.Init.Act)
		nrn.GeSyn -= decay * (nrn.GeSyn - nrn.GeBase)
		nrn.Ge -= decay * (nrn.Ge - nrn.GeBase)
		nrn.Gi -= decay * (nrn.Gi - nrn.GiBase)
		nrn.Gk -= decay * nrn.Gk

		nrn.Vm -= decay * (nrn.Vm - ac.Init.Vm)

		nrn.GeNoise -= decay * nrn.GeNoise
		nrn.GiNoise -= decay * nrn.GiNoise

		nrn.GiSyn -= decay * nrn.GiSyn
	}

	nrn.VmDend -= glong * (nrn.VmDend - ac.Init.Vm)

	nrn.MahpN -= ac.Decay.AHP * nrn.MahpN
	nrn.SahpCa -= ac.Decay.AHP * nrn.SahpCa
	nrn.SahpN -= ac.Decay.AHP * nrn.SahpN
	nrn.GknaMed -= ac.Decay.AHP * nrn.GknaMed
	nrn.GknaSlow -= ac.Decay.AHP * nrn.GknaSlow

	nrn.GgabaB -= glong * nrn.GgabaB
	nrn.GABAB -= glong * nrn.GABAB
	nrn.GABABx -= glong * nrn.GABABx

	nrn.Gvgcc -= glong * nrn.Gvgcc
	nrn.VgccM -= glong * nrn.VgccM
	nrn.VgccH -= glong * nrn.VgccH
	nrn.Gak -= glong * nrn.Gak

	nrn.GnmdaSyn -= glong * nrn.GnmdaSyn
	nrn.Gnmda -= glong * nrn.Gnmda

	// learning-based NMDA, Ca values decayed in Learn.DecayNeurCa

	nrn.Inet = 0
	nrn.GeRaw = 0
	nrn.GiRaw = 0
	nrn.SSGi = 0
	nrn.SSGiDend = 0
	nrn.GeExt = 0
}

// InitActs initializes activation state in neuron -- called during InitWts but otherwise not
// automatically called (DecayState is used instead)
func (ac *ActParams) InitActs(nrn *Neuron) {
	nrn.Spike = 0
	nrn.Spiked = 0
	nrn.ISI = -1
	nrn.ISIAvg = -1
	nrn.Act = ac.Init.Act
	nrn.ActInt = ac.Init.Act
	nrn.GeBase = 0
	nrn.GiBase = 0
	nrn.GeSyn = nrn.GeBase
	nrn.Ge = nrn.GeBase
	nrn.Gi = nrn.GiBase
	nrn.Gk = 0
	nrn.Inet = 0
	nrn.Vm = ac.Init.Vm
	nrn.VmDend = ac.Init.Vm
	nrn.Target = 0
	nrn.Ext = 0

	nrn.SpkMaxCa = 0
	nrn.SpkMax = 0
	nrn.Attn = 1
	nrn.RLRate = 1

	nrn.GeNoiseP = 1
	nrn.GeNoise = 0
	nrn.GiNoiseP = 1
	nrn.GiNoise = 0

	nrn.GiSyn = 0

	nrn.MahpN = 0
	nrn.SahpCa = 0
	nrn.SahpN = 0
	nrn.GknaMed = 0
	nrn.GknaSlow = 0

	nrn.GnmdaSyn = 0
	nrn.Gnmda = 0
	nrn.SnmdaO = 0
	nrn.SnmdaI = 0

	nrn.GgabaB = 0
	nrn.GABAB = 0
	nrn.GABABx = 0

	nrn.Gvgcc = 0
	nrn.VgccM = 0
	nrn.VgccH = 0
	nrn.Gak = 0

	nrn.GeRaw = 0
	nrn.GiRaw = 0
	nrn.SSGi = 0
	nrn.SSGiDend = 0
	nrn.GeExt = 0

	ac.InitLongActs(nrn)
}

// InitLongActs initializes longer time-scale activation states in neuron
// (SpkPrv, SpkSt*, ActM, ActP, GeM)
// Called from InitActs, which is called from InitWts,
// but otherwise not automatically called
// (DecayState is used instead)
func (ac *ActParams) InitLongActs(nrn *Neuron) {
	nrn.SpkPrv = 0
	nrn.SpkSt1 = 0
	nrn.SpkSt2 = 0
	nrn.ActM = 0
	nrn.ActP = 0
	nrn.GeM = 0
}

///////////////////////////////////////////////////////////////////////
//  Cycle

// NMDAFromRaw updates all the NMDA variables from
// total Ge (GeRaw + Ext) and current Vm, Spiking
func (ac *ActParams) NMDAFromRaw(nrn *Neuron, geTot float32) {
	if geTot < 0 {
		geTot = 0
	}
	nrn.GnmdaSyn = ac.NMDA.NMDASyn(nrn.GnmdaSyn, geTot)
	nrn.Gnmda = ac.NMDA.Gnmda(nrn.GnmdaSyn, nrn.VmDend)
	// note: nrn.NmdaCa computed via Learn.LrnNMDA in learn.go, CaM method
}

// GvgccFromVm updates all the VGCC voltage-gated calcium channel variables
// from VmDend
func (ac *ActParams) GvgccFromVm(nrn *Neuron) {
	nrn.Gvgcc = ac.VGCC.Gvgcc(nrn.VmDend, nrn.VgccM, nrn.VgccH)
	var dm, dh float32
	ac.VGCC.DMHFromV(nrn.VmDend, nrn.VgccM, nrn.VgccH, &dm, &dh)
	nrn.VgccM += dm
	nrn.VgccH += dh
	nrn.VgccCa = ac.VGCC.CaFromG(nrn.VmDend, nrn.Gvgcc, nrn.VgccCa) // note: may be overwritten!
}

// GkFromVm updates all the Gk-based conductances: Mahp, KNa, Gak
func (ac *ActParams) GkFromVm(nrn *Neuron) {
	dn := ac.Mahp.DNFromV(nrn.Vm, nrn.MahpN)
	nrn.MahpN += dn
	nrn.Gak = ac.AK.Gak(nrn.VmDend)
	nrn.Gk = nrn.Gak + ac.Mahp.GmAHP(nrn.MahpN) + ac.Sahp.GsAHP(nrn.SahpN)
	if ac.KNa.On.IsTrue() {
		ac.KNa.GcFromSpike(&nrn.GknaMed, &nrn.GknaSlow, nrn.Spike > .5)
		nrn.Gk += nrn.GknaMed + nrn.GknaSlow
	}
}

// GeFromSyn integrates Ge excitatory conductance from GeSyn.
// geExt is extra conductance to add to the final Ge value
func (ac *ActParams) GeFromSyn(ni int, nrn *Neuron, geSyn, geExt float32, randctr *sltype.Uint2) {
	nrn.GeExt = 0
	if ac.Clamp.Add.IsTrue() && nrn.HasFlag(NeuronHasExt) {
		nrn.GeExt = nrn.Ext * ac.Clamp.Ge
		geSyn += nrn.GeExt
	}
	geSyn = ac.Attn.ModValue(geSyn, nrn.Attn)

	if ac.Clamp.Add.IsTrue() && nrn.HasFlag(NeuronHasExt) {
		geSyn = nrn.Ext * ac.Clamp.Ge
		nrn.GeExt = geSyn
		geExt = 0 // no extra in this case
	}

	nrn.Ge = geSyn + geExt
	if nrn.Ge < 0 {
		nrn.Ge = 0
	}
	ac.GeNoise(ni, nrn, randctr)
}

// GeNoise updates nrn.GeNoise if active
func (ac *ActParams) GeNoise(ni int, nrn *Neuron, randctr *sltype.Uint2) {
	if slbool.IsFalse(ac.Noise.On) || ac.Noise.Ge == 0 {
		return
	}
	ge := ac.Noise.PGe(&nrn.GeNoiseP, ni, randctr)
	nrn.GeNoise = ac.Dt.GeSynFromRaw(nrn.GeNoise, ge)
	nrn.Ge += nrn.GeNoise
}

// GiNoise updates nrn.GiNoise if active
func (ac *ActParams) GiNoise(ni int, nrn *Neuron, randctr *sltype.Uint2) {
	if slbool.IsFalse(ac.Noise.On) || ac.Noise.Gi == 0 {
		return
	}
	gi := ac.Noise.PGi(&nrn.GiNoiseP, ni, randctr)
	// fmt.Printf("rc: %v\n", *randctr)
	nrn.GiNoise = ac.Dt.GiSynFromRaw(nrn.GiNoise, gi)
}

// GiFromSyn integrates GiSyn inhibitory synaptic conductance from GiRaw value
// (can add other terms to geRaw prior to calling this)
func (ac *ActParams) GiFromSyn(ni int, nrn *Neuron, giSyn float32, randctr *sltype.Uint2) float32 {
	ac.GiNoise(ni, nrn, randctr)
	if giSyn < 0 { // negative inhib G doesn't make any sense
		giSyn = 0
	}
	return giSyn
}

// InetFromG computes net current from conductances and Vm
func (ac *ActParams) InetFromG(vm, ge, gl, gi, gk float32) float32 {
	inet := ge*(ac.Erev.E-vm) + gl*ac.Gbar.L*(ac.Erev.L-vm) + gi*(ac.Erev.I-vm) + gk*(ac.Erev.K-vm)
	if inet > ac.Dt.VmTau {
		inet = ac.Dt.VmTau
	} else if inet < -ac.Dt.VmTau {
		inet = -ac.Dt.VmTau
	}
	return inet
}

// VmFromInet computes new Vm value from inet, clamping range
func (ac *ActParams) VmFromInet(vm, dt, inet float32) float32 {
	return ac.VmRange.ClipValue(vm + dt*inet)
}

// VmInteg integrates Vm over VmSteps to obtain a more stable value
// Returns the new Vm and inet values.
func (ac *ActParams) VmInteg(vm, dt, ge, gl, gi, gk float32, nvm, inet *float32) {
	dt *= ac.Dt.DtStep
	*nvm = vm
	for i := int32(0); i < ac.Dt.VmSteps; i++ {
		*inet = ac.InetFromG(*nvm, ge, gl, gi, gk)
		*nvm = ac.VmFromInet(*nvm, dt, *inet)
	}
}

// VmFromG computes membrane potential Vm from conductances Ge, Gi, and Gk.
func (ac *ActParams) VmFromG(nrn *Neuron) {
	updtVm := true
	// note: nrn.ISI has NOT yet been updated at this point: 0 right after spike, etc
	// so it takes a full 3 time steps after spiking for Tr period
	if ac.Spike.Tr > 0 && nrn.ISI >= 0 && nrn.ISI < float32(ac.Spike.Tr) {
		updtVm = false // don't update the spiking vm during refract
	}

	ge := nrn.Ge * ac.Gbar.E
	gi := nrn.Gi * ac.Gbar.I
	gk := nrn.Gk * ac.Gbar.K
	var nvm, inet, exVm, expi float32
	if updtVm {
		ac.VmInteg(nrn.Vm, ac.Dt.VmDt, ge, 1, gi, gk, &nvm, &inet)
		if updtVm && slbool.IsTrue(ac.Spike.Exp) { // add spike current if relevant
			exVm = 0.5 * (nvm + nrn.Vm) // midpoint for this
			expi = ac.Gbar.L * ac.Spike.ExpSlope *
				math32.FastExp((exVm-ac.Spike.Thr)/ac.Spike.ExpSlope)
			if expi > ac.Dt.VmTau {
				expi = ac.Dt.VmTau
			}
			inet += expi
			nvm = ac.VmFromInet(nvm, ac.Dt.VmDt, expi)
		}
		nrn.Vm = nvm
		nrn.Inet = inet
	} else { // decay back to VmR
		var dvm float32
		if int32(nrn.ISI) == ac.Spike.Tr-1 {
			dvm = (ac.Spike.VmR - nrn.Vm)
		} else {
			dvm = ac.Spike.RDt * (ac.Spike.VmR - nrn.Vm)
		}
		nrn.Vm = nrn.Vm + dvm
		nrn.Inet = dvm * ac.Dt.VmTau
	}

	{ // always update VmDend
		glEff := float32(1)
		if !updtVm {
			glEff += ac.Dend.GbarR
		}
		giEff := gi + ac.Gbar.I*nrn.SSGiDend
		ac.VmInteg(nrn.VmDend, ac.Dt.VmDendDt, ge, glEff, giEff, gk, &nvm, &inet)
		if updtVm {
			nvm = ac.VmFromInet(nvm, ac.Dt.VmDendDt, ac.Dend.GbarExp*expi)
		}
		nrn.VmDend = nvm
	}
}

// SpikeFromG computes Spike from Vm and ISI-based activation
func (ac *ActParams) SpikeFromVm(nrn *Neuron) {
	var thr float32
	if slbool.IsTrue(ac.Spike.Exp) {
		thr = ac.Spike.ExpThr
	} else {
		thr = ac.Spike.Thr
	}
	if nrn.Vm >= thr {
		nrn.Spike = 1
		if nrn.ISIAvg == -1 {
			nrn.ISIAvg = -2
		} else if nrn.ISI > 0 { // must have spiked to update
			ac.Spike.AvgFromISI(&nrn.ISIAvg, nrn.ISI+1)
		}
		nrn.ISI = 0
	} else {
		nrn.Spike = 0
		if nrn.ISI >= 0 {
			nrn.ISI += 1
			if nrn.ISI < 10 {
				nrn.Spiked = 1
			} else {
				nrn.Spiked = 0
			}
		} else {
			nrn.Spiked = 0
		}
		if nrn.ISIAvg >= 0 && nrn.ISI > 0 && nrn.ISI > 1.2*nrn.ISIAvg {
			ac.Spike.AvgFromISI(&nrn.ISIAvg, nrn.ISI)
		}
	}

	nwAct := ac.Spike.ActFromISI(nrn.ISIAvg, .001, ac.Dt.Integ)
	if nwAct > 1 {
		nwAct = 1
	}
	nwAct = nrn.Act + ac.Dt.VmDt*(nwAct-nrn.Act)
	nrn.Act = nwAct
}

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
	Dt CaDtParams `view:"inline"`

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
// CaSpk* values are integrated separately at the Neuron level and used for UpdateThr
// and RLRate as a proxy for the activation (spiking) based learning signal.
type CaSpkParams struct {

	// gain multiplier on spike for computing CaSpk: increasing this directly affects the magnitude of the trace values, learning rate in Target layers, and other factors that depend on CaSpk values: RLRate, UpdateThr.  Prjn.KinaseCa.SpikeG provides an additional gain factor specific to the synapse-level trace factors, without affecting neuron-level CaSpk values.  Larger networks require higher gain factors at the neuron level -- 12, vs 8 for smaller.
	SpikeG float32 `default:"8,12"`

	// time constant for integrating spike-driven calcium trace at sender and recv neurons, CaSyn, which then drives synapse-level integration of the joint pre * post synapse-level activity, in cycles (msec)
	SynTau float32 `default:"30" min:"1"`

	// rate = 1 / tau
	SynDt float32 `view:"-" json:"-" xml:"-" edit:"-"`

	// Ca gain factor for SynSpkCa learning rule, to compensate for the effect of SynTau, which increases Ca as it gets larger.  is 1 for SynTau = 30 -- todo: eliminate this at some point!
	SynSpkG float32 `view:"+" json:"-" xml:"-" edit:"-"`

	// time constants for integrating CaSpk across M, P and D cascading levels -- these are typically the same as in CaLrn and Prjn level for synaptic integration, except for the M factor.
	Dt CaDtParams `view:"inline"`
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
	np.SynSpkG = math32.Sqrt(30) / math32.Sqrt(np.SynTau)
}

// CaFromSpike computes CaSpk* and CaSyn calcium signals based on current spike.
func (np *CaSpkParams) CaFromSpike(nrn *Neuron) {
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
	ErrLRate float32 `default:"0.02"`

	// rate parameter for how much to scale synaptic weights in proportion to the AvgDif between target and actual proportion activity -- this determines the effective strength of the constraint, and larger models may need more than the weaker default value.
	SynScaleRate float32 `default:"0.005,0.0002"`

	// amount of mean trg change to subtract -- 1 = full zero sum.  1 works best in general -- but in some cases it may be better to start with 0 and then increase using network SetSubMean method at a later point.
	SubMean float32 `default:"0,1"`

	// permute the order of TrgAvg values within layer -- otherwise they are just assigned in order from highest to lowest for easy visualization -- generally must be true if any topographic weights are being used
	Permute slbool.Bool `default:"true"`

	// use pool-level target values if pool-level inhibition and 4D pooled layers are present -- if pool sizes are relatively small, then may not be useful to distribute targets just within pool
	Pool slbool.Bool

	pad, pad1 float32

	// range of target normalized average activations -- individual neurons are assigned values within this range to TrgAvg, and clamped within this range.
	TrgRange F32 `default:"{0.5 2}"`
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
	mx := math32.Max(scap, scad)
	if mx > rl.SpkThr { // avoid div by 0
		dif := math32.Abs(scap - scad)
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

	// parameterizes the neuron-level spike-driven calcium signals, starting with CaSyn that is integrated at the neuron level, and drives synapse-level, pre * post Ca integration, which provides the Tr trace that multiplies error signals, and drives learning directly for Target layers. CaSpk* values are integrated separately at the Neuron level and used for UpdateThr and RLRate as a proxy for the activation (spiking) based learning signal.
	CaSpk CaSpkParams `view:"inline"`

	// NMDA channel parameters used for learning, vs. the ones driving activation -- allows exploration of learning parameters independent of their effects on active maintenance contributions of NMDA, and may be supported by different receptor subtypes
	LrnNMDA NMDAParams `view:"inline"`

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

// LrnNMDAFromRaw updates the separate NMDA conductance and calcium values
// based on GeTot = GeRaw + external ge conductance.  These are the variables
// that drive learning -- can be the same as activation but also can be different
// for testing learning Ca effects independent of activation effects.
func (ln *LearnNeurParams) LrnNMDAFromRaw(nrn *Neuron, geTot float32) {
	if geTot < 0 {
		geTot = 0
	}
	nrn.GnmdaLrn = ln.LrnNMDA.NMDASyn(nrn.GnmdaLrn, geTot)
	gnmda := ln.LrnNMDA.Gnmda(nrn.GnmdaLrn, nrn.VmDend)
	nrn.NmdaCa = gnmda * ln.LrnNMDA.CaFromV(nrn.VmDend)
	ln.LrnNMDA.SnmdaFromSpike(nrn.Spike, &nrn.SnmdaO, &nrn.SnmdaI)
}

// CaFromSpike updates all spike-driven calcium variables, including CaLrn and CaSpk.
// Computed after new activation for current cycle is updated.
func (ln *LearnNeurParams) CaFromSpike(nrn *Neuron) {
	ln.CaSpk.CaFromSpike(nrn)
	ln.CaLrn.CaLrn(nrn)
}

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
