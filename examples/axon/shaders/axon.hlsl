#ifndef __AXON_HLSL__
#define __AXON_HLSL__


#include "fastexp.hlsl"

static const float MaxFloat32 = 3.402823466e+38;
static const float MinFloat32 = 1.175494351e-38;

// F32 represents a min / max range for float values.
// Supports clipping, renormalizing, etc
struct F32 {
	float Min;
	float Max;

	float pad, pad1;
	bool IsValid() { return this.Min <= this.Max; }

	bool InRange(float val) {
		return ((val >= this.Min) && (val <= this.Max));
	}

	bool IsLow(float val) {
		return (val < this.Min);
	}

	bool IsHigh(float val) {
		return (val > this.Min);
	}

// iteratively calling Fit*InRange
	void SetInfinity() {
		this.Min = MaxFloat32;
		this.Max = -MaxFloat32;
	}

	float Range() {
		return this.Max - this.Min;
	}

	float Scale() {
		float r;
		r = this.Range();
		if (r != 0) {
			return 1.0 / r;
		}
		return 0;
	}

	float Midpoint() {
		return 0.5 * (this.Max + this.Min);
	}

// Clips the value within Min-Max range first.
	float NormValue(float val) {
		return (this.ClipValue(val) - this.Min) * this.Scale();
	}

	float ProjValue(float val) {
		return this.Min + (val * this.Range());
	}

// Note: a NaN will remain as a NaN
	float ClipValue(float val) {
		if (val < this.Min) {
			return this.Min;
		}
		if (val > this.Max) {
			return this.Max;
		}
		return val;
	}

// Note: a NaN will remain as a NaN
	float ClipNormValue(float val) {
		if (val < this.Min) {
			return 0;
		}
		if (val > this.Max) {
			return 1;
		}
		return this.NormValue(val);
	}

// returns true if we had to adjust to fit.
	bool FitValInRange(float val) {
		bool adj;
		adj = false;
		if (val < this.Min) {
			this.Min = val;
			adj = true;
		}
		if (val > this.Max) {
			this.Max = val;
			adj = true;
		}
		return adj;
	}

	void Set(float min, float max) {
		this.Min = min;
		this.Max = max;
	}

};

// Chans are ion channels used in computing point-neuron activation function
struct Chans {

	// excitatory sodium (Na) AMPA channels activated by synaptic glutamate
	float E;

	// constant leak (potassium, K+) channels -- determines resting potential (typically higher than resting potential of K)
	float L;

	// inhibitory chloride (Cl-) channels activated by synaptic GABA
	float I;

	// gated / active potassium channels -- typically hyperpolarizing relative to leak / rest
	float K;
};

// VToBio returns biological mV voltage from normalized 0-1 voltage
// where 0 = -100mV and 1 = 0mV
float VToBio(float vm) {
	return vm*100 - 100;
}

// VFromBio returns normalized 0-1 voltage from biological mV voltage
// where 0 = -100mV and 1 = 0mV
float VFromBio(float vm) {
	return (vm + 100) / 100;
}

// AKsParams provides a highly simplified stateless A-type K channel
// that only has the voltage-gated activation (M) dynamic with a cutoff
// that ends up capturing a close approximation to the much more complex AK function.
// This is voltage gated with maximal activation around -37 mV.
// It is particularly important for counteracting the excitatory effects of
// voltage gated calcium channels which can otherwise drive runaway excitatory currents.
struct AKsParams {

	// strength of AK current
	float Gbar;

	// H factor as a constant multiplier on overall M factor result -- rescales M to level consistent with H being present at full strength
	float Hf;

	// multiplier for M -- determines slope of function
	float Mf;

	// voltage offset in biological units for M function
	float Voff;
	float Vmax;

	float pad, pad1, pad2;
	float MFromV(float vbio) {
		if (vbio > this.Vmax) {
			vbio = this.Vmax;
		}
		return this.Hf / (1.0 + FastExp(-this.Mf*(vbio+this.Voff)));
	}

	float MFromVnorm(float v) {
		return this.MFromV(VToBio(v));
	}

// GBar * MFromVnorm(v)
	float Gak(float v) {
		return this.Gbar * this.MFromVnorm(v);
	}

};

// Defaults sets the parameters for distal dendrites

// GABABParams control the GABAB dynamics in PFC Maint neurons,
// based on Brunel & Wang (2001) parameters.
struct GABABParams {

	// overall strength multiplier of GABA-B current
	float Gbar;

	// rise time for bi-exponential time dynamics of GABA-B
	float RiseTau;

	// decay time for bi-exponential time dynamics of GABA-B
	float DecayTau;

	// baseline level of GABA-B channels open independent of inhibitory input (is added to spiking-produced conductance)
	float Gbase;

	// multiplier for converting Gi to equivalent GABA spikes
	float GiSpike;

	// time offset when peak conductance occurs, in msec, computed from RiseTau and DecayTau
	float MaxTime;

	// time constant factor used in integration: (Decay / Rise) ^ (Rise / (Decay - Rise))
	float TauFact;

	float pad;
	float GFromV(float v) {
		float vbio;
		vbio = VToBio(v);
		if (vbio < -90) {
			vbio = -90;
		}
		return 1.0 / (1.0 + FastExp(0.1*((vbio+90)+10)));
	}

// based on normalized spiking factor (i.e., Gi from FFFB etc)
	float GFromS(float s) {
		float ss;
		ss = s * this.GiSpike;
		if (ss > 10) {
			ss = 10;
		}
		return 1.0 / (1.0 + FastExp(-(ss-7.1)/1.4));
	}

	float DG(float g, float x) {
		return (this.TauFact*x - g) / this.RiseTau;
	}

	float DX(float x) {
		return -x / this.DecayTau;
	}

// based on current values and gi inhibitory conductance (proxy for GABA spikes)
	float GFromGX(float gabaB, float gabaBx) {
		return gabaB + this.DG(gabaB, gabaBx);
	}

// based on current values and gi inhibitory conductance (proxy for GABA spikes)
	float XFromGiX(float gabaBx, float gi) {
		return gabaBx + this.GFromS(gi) + this.DX(gabaBx);
	}

// Gbar, Gbase, and voltage-gating
	float GgabaB(float gabaB, float vm) {
		return this.Gbar * this.GFromV(vm) * (gabaB + this.Gbase);
	}

};

// KNaParams implements sodium (Na) gated potassium (K) currents
// that drive adaptation (accommodation) in neural firing.
// As neurons spike, driving an influx of Na, this activates
// the K channels, which, like leak channels, pull the membrane
// potential back down toward rest (or even below).
struct KNaParams {

	// if On, use this component of K-Na adaptation
	int On;

	// Rise rate of fast time-scale adaptation as function of Na concentration due to spiking -- directly multiplies -- 1/rise = tau for rise rate
	float Rise;

	// Maximum potential conductance of fast K channels -- divide nA biological value by 10 for the normalized units here
	float Max;

	// time constant in cycles for decay of adaptation, which should be milliseconds typically (tau is roughly how long it takes for value to change significantly -- 1.4x the half-life)
	float Tau;

	// 1/Tau rate constant
	float Dt;

	float pad, pad1, pad2;
	void GcFromSpike(inout float gKNa, bool spike) {
		if ((1 == this.On)) {
			if (spike) {
				gKNa += this.Rise * (this.Max - gKNa);
			} else {
				gKNa -= this.Dt * gKNa;
			}
		} else {
			gKNa = 0;
		}
	}

};

// KNaMedSlow describes sodium-gated potassium channel adaptation mechanism.
// Evidence supports 2 different time constants:
// Slick (medium) and Slack (slow)
struct KNaMedSlow {

	// if On, apply K-Na adaptation
	int On;

	float pad, pad1, pad2;

	// medium time-scale adaptation
	KNaParams Med;

	// slow time-scale adaptation
	KNaParams Slow;
	void GcFromSpike(inout float gKNaM, inout float gKNaS, bool spike) {
		this.Med.GcFromSpike(gKNaM, spike);
		this.Slow.GcFromSpike(gKNaS, spike);
	}

};

// MahpParams implements an M-type medium afterhyperpolarizing (mAHP) channel,
// where m also stands for muscarinic due to the ACh inactivation of this channel.
// It has a slow activation and deactivation time constant, and opens at a lowish
// membrane potential.
// There is one gating variable n updated over time with a tau that is also voltage dependent.
// The infinite-time value of n is voltage dependent according to a logistic function
// of the membrane potential, centered at Voff with slope Vslope.
struct MahpParams {

	// strength of mAHP current
	float Gbar;

	// voltage offset (threshold) in biological units for infinite time N gating function -- where the gate is at 50% strength
	float Voff;

	// slope of the arget (infinite time) gating function
	float Vslope;

	// maximum slow rate time constant in msec for activation / deactivation.  The effective Tau is much slower -- 1/20th in original temp, and 1/60th in standard 37 C temp
	float TauMax;

	// temperature adjustment factor: assume temp = 37 C, whereas original units were at 23 C
	float Tadj;

	// 1/Tau
	float DtMax;
	float pad, pad1;
	float EFun(float z) {
		if (abs(z) < 1.0e-4) {
			return 1.0 - 0.5*z;
		}
		return z / (FastExp(z) - 1.0);
	}

// voltage-dependent time constant tau, from vbio
	void NinfTauFromV(float vbio, inout float ninf, inout float tau) {
		float vo, a, b;
		vo = vbio - this.Voff;

		// logical functions, but have signularity at Voff (vo = 0)
		// a := mp.DtMax * vo / (1.0 - FastExp(-vo/mp.Vslope))
		// b := -mp.DtMax * vo / (1.0 - FastExp(vo/mp.Vslope))

		a = this.DtMax * this.Vslope * this.EFun(-vo/this.Vslope);
		b = this.DtMax * this.Vslope * this.EFun(vo/this.Vslope);
		tau = 1.0 / (a + b);
		ninf = a * tau;   // a / (a+b)
		tau /= this.Tadj; // correct right away..
	}

// voltage-dependent time constant tau, from normalized vm
	void NinfTauFromVnorm(float v, inout float ninf, inout float tau) {
		this.NinfTauFromV(VToBio(v), ninf, tau);
	}

	float DNFromV(float v, float n) {
		float ninf, tau;
		this.NinfTauFromVnorm(v, ninf, tau);
		// dt := 1.0 - FastExp(-mp.Tadj/tau) // Mainen comments out this form; Poirazi uses
		// dt := mp.Tadj / tau // simple linear fix
		return (ninf - n) / tau;
	}

	float GmAHP(float n) {
		return this.Tadj * this.Gbar * n;
	}

};

// Defaults sets the parameters

// 3.2 basically

// NMDAParams control the NMDA dynamics, based on Jahr & Stevens (1990) equations
// which are widely used in models, from Brunel & Wang (2001) to Sanders et al. (2013).
// The overall conductance is a function of a voltage-dependent postsynaptic factor based
// on Mg ion blockage, and presynaptic Glu-based opening, which in a simple model just
// increments
struct NMDAParams {

	// overall multiplier for strength of NMDA current -- multiplies GnmdaSyn to get net conductance.  0.15 standard for SnmdaDeplete = false, 1.4 when on.
	float Gbar;

	// decay time constant for NMDA channel activation  -- rise time is 2 msec and not worth extra effort for biexponential.  30 fits the Urakubo et al (2008) model with ITau = 100, but 100 works better in practice is small networks so far.
	float Tau;

	// decay time constant for NMDA channel inhibition, which captures the Urakubo et al (2008) allosteric dynamics (100 fits their model well) -- set to 1 to eliminate that mechanism.
	float ITau;

	// magnesium ion concentration: Brunel & Wang (2001) and Sanders et al (2013) use 1 mM, based on Jahr & Stevens (1990). Urakubo et al (2008) use 1.5 mM. 1.4 with Voff = 5 works best so far in large models, 1.2, Voff = 0 best in smaller nets.
	float MgC;

	// offset in membrane potential in biological units for voltage-dependent functions.  5 corresponds to the -65 mV rest, -45 threshold of the Urakubo et al (2008) model.  0 is best in small models
	float Voff;

	// rate = 1 / tau
	float Dt;

	// rate = 1 / tau
	float IDt;

	// MgFact = MgC / 3.57
	float MgFact;
// based on Mg ion blocking
	float MgGFromVbio(float vbio) {
		vbio += this.Voff;
		if (vbio >= 0) {
			return 0;
		}
		return 1.0 / (1.0 + this.MgFact*FastExp(-0.062*vbio));
	}

// based on Mg ion blocking
	float MgGFromV(float v) {
		return this.MgGFromVbio(VToBio(v));
	}

// potential -- this factor is needed for computing the calcium current * MgGFromV.
// This is the same function used in VGCC for their conductance factor.
	float CaFromVbio(float vbio) {
		vbio += this.Voff;
		if (vbio > -0.1 && vbio < 0.1) {
			return 1.0 / (0.0756 + 0.5*vbio);
		}
		return -vbio / (1.0 - FastExp(0.0756*vbio));
	}

// potential -- this factor is needed for computing the calcium current * MgGFromV
	float CaFromV(float v) {
		return this.CaFromVbio(VToBio(v));
	}

// based on new raw spike-driven Glu binding.
	float NMDASyn(float nmda, float raw) {
		return nmda + raw - this.Dt*nmda;
	}

// including the GBar factor
	float Gnmda(float nmda, float vm) {
		return this.Gbar * this.MgGFromV(vm) * nmda;
	}

// using the inhibition and decay factors.  These dynamics closely match the
// Urakubo et al (2008) allosteric NMDA receptor behavior, with ITau = 100, Tau = 30
	void SnmdaFromSpike(float spike, inout float snmdaO, inout float snmdaI) {
		if (spike > 0) {
			float inh;
			inh = (1 - snmdaI);
			snmdaO += inh * (1 - snmdaO);
			snmdaI += inh;
		} else {
			snmdaO -= this.Dt * snmdaO;
			snmdaI -= this.IDt * snmdaI;
		}
	}

};

// off by default, as it doesn't work in actual axon models..

// SahpParams implements a slow afterhyperpolarizing (sAHP) channel,
// It has a slowly accumulating calcium value, aggregated at the
// theta cycle level, that then drives the logistic gating function,
// so that it only activates after a significant accumulation.
// After which point it decays.
// For the theta-cycle updating, the normal m-type tau is all within
// the scope of a single theta cycle, so we just omit the time integration
// of the n gating value, but tau is computed in any case.
struct SahpParams {

	// strength of sAHP current
	float Gbar;

	// time constant for integrating Ca across theta cycles
	float CaTau;

	// integrated Ca offset (threshold) for infinite time N gating function -- where the gate is at 50% strength
	float Off;

	// slope of the infinite time logistic gating function
	float Slope;

	// maximum slow rate time constant in msec for activation / deactivation.  The effective Tau is much slower -- 1/20th in original temp, and 1/60th in standard 37 C temp
	float TauMax;

	// 1/Tau
	float CaDt;

	// 1/Tau
	float DtMax;
	float pad;
	float EFun(float z) {
		if (abs(z) < 1.0e-4) {
			return 1.0 - 0.5*z;
		}
		return z / (FastExp(z) - 1.0);
	}

// time constant tau, from integrated Ca value
	void NinfTauFromCa(float ca, inout float ninf, inout float tau) {
		float co, a, b;
		co = ca - this.Off;

		// logical functions, but have signularity at Voff (vo = 0)
		// a := mp.DtMax * vo / (1.0 - FastExp(-vo/mp.Vslope))
		// b := -mp.DtMax * vo / (1.0 - FastExp(vo/mp.Vslope))

		a = this.DtMax * this.Slope * this.EFun(-co/this.Slope);
		b = this.DtMax * this.Slope * this.EFun(co/this.Slope);
		tau = 1.0 / (a + b);
		ninf = a * tau; // a / (a+b)
	}

	float CaInt(float caInt, float ca) {
		caInt += this.CaDt * (ca - caInt);
		return caInt;
	}

// Omit this and just use ninf directly for theta-cycle updating.
	float DNFromV(float ca, float n) {
		float ninf, tau;
		this.NinfTauFromCa(ca, ninf, tau);
		return (ninf - n) / tau;
	}

	float GsAHP(float n) {
		return this.Gbar * n;
	}

};

// Defaults sets the parameters

// VGCCParams control the standard L-type Ca channel
struct VGCCParams {

	// strength of VGCC current -- 0.12 value is from Urakubo et al (2008) model -- best fits actual model behavior using axon equations (1.5 nominal in that model), 0.02 works better in practice for not getting stuck in high plateau firing
	float Gbar;

	// calcium from conductance factor -- important for learning contribution of VGCC
	float Ca;

	float pad, pad1;
	float GFromV(float v) {
		float vbio;
		vbio = VToBio(v);
		if (vbio > -0.1 && vbio < 0.1) {
			return 1.0 / (0.0756 + 0.5*vbio);
		}
		return -vbio / (1.0 - FastExp(0.0756*vbio));
	}

	float MFromV(float vbio) {
		if (vbio < -60) {
			return 0;
		}
		if (vbio > -10) {
			return 1;
		}
		return 1.0 / (1.0 + FastExp(-(vbio + 37)));
	}

	float HFromV(float vbio) {
		if (vbio < -50) {
			return 1;
		}
		if (vbio > -10) {
			return 0;
		}
		return 1.0 / (1.0 + FastExp((vbio+41)*2));
	}

// as a function of V normalized (0-1)
	void DMHFromV(float v, float m, float h, inout float dm, inout float dh) {
		float vbio;
		vbio = VToBio(v);
		if (vbio > 0) {
			vbio = 0;
		}
		dm = (this.MFromV(vbio) - m) / 3.6;
		dh = (this.HFromV(vbio) - h) / 29.0;
	}

	float Gvgcc(float vm, float m, float h) {
		return this.Gbar * this.GFromV(vm) * m * m * m * h;
	}

// and normalized membrane potential.
	float CaFromG(float v, float g, float ca) {
		float vbio;
		vbio = VToBio(v);
		return -vbio * this.Ca * g;
	}

};

// CaDtParams has rate constants for integrating Ca calcium
// at different time scales, including final CaP = CaMKII and CaD = DAPK1
// timescales for LTP potentiation vs. LTD depression factors.
struct CaDtParams {

	// CaM (calmodulin) time constant in cycles (msec) -- for synaptic-level integration this integrates on top of Ca signal from send->CaSyn * recv->CaSyn, each of which are typically integrated with a 30 msec Tau.
	float MTau;

	// LTP spike-driven Ca factor (CaP) time constant in cycles (msec), simulating CaMKII in the Kinase framework, with 40 on top of MTau roughly tracking the biophysical rise time.  Computationally, CaP represents the plus phase learning signal that reflects the most recent past information.
	float PTau;

	// LTD spike-driven Ca factor (CaD) time constant in cycles (msec), simulating DAPK1 in Kinase framework.  Computationally, CaD represents the minus phase learning signal that reflects the expectation representation prior to experiencing the outcome (in addition to the outcome).
	float DTau;

	// rate = 1 / tau
	float MDt;

	// rate = 1 / tau
	float PDt;

	// rate = 1 / tau
	float DDt;

	float pad, pad1;
};

// CaParams has rate constants for integrating spike-driven Ca calcium
// at different time scales, including final CaP = CaMKII and CaD = DAPK1
// timescales for LTP potentiation vs. LTD depression factors.
struct CaParams {

	// spiking gain factor for SynSpk learning rule variants.  This alters the overall range of values, keeping them in roughly the unit scale, and affects effective learning rate.
	float SpikeG;

	// IMPORTANT: only used for SynSpkTheta learning mode: threshold on Act value for updating synapse-level Ca values -- this is purely a performance optimization that excludes random infrequent spikes -- 0.05 works well on larger networks but not smaller, which require the .01 default.
	float UpdateThr;

	// maximum ISI for integrating in Opt mode -- above that just set to 0
	int MaxISI;

	float pad;

	// time constants for integrating at M, P, and D cascading levels
	CaDtParams Dt;
// The SpikeG factor determines strength of increase to CaM.
	void FromSpike(float spike, inout float caM, inout float caP, inout float caD) {
		caM += this.Dt.MDt * (this.SpikeG*spike - caM);
		caP += this.Dt.PDt * (caM - caP);
		caD += this.Dt.DDt * (caP - caD);
	}

// The SpikeG factor is NOT applied to Ca and should be pre-applied
// as appropriate.
	void FromCa(float ca, inout float caM, inout float caP, inout float caD) {
		caM += this.Dt.MDt * (ca - caM);
		caP += this.Dt.PDt * (caM - caP);
		caD += this.Dt.DDt * (caP - caD);
	}

// and last update time, which is -1 if never updated
// (in which case return is -1)
	int IntFromTime(int ctime, int utime) {
		if (utime < 0) {
			return -1;
		}
		return ctime - utime;
	}

// optimized spike-time update versions.
// ctime is current time in msec, and utime is last update time (-1 if never)
	void CurCa(int ctime, int utime, inout float caM, inout float caP, inout float caD) {
		int isi = this.IntFromTime(ctime, utime);
		if (isi <= 0) {
			return;
		}
		if (isi > this.MaxISI) {
			caM = 0;
			caP = 0;
			caD = 0;
			return;
		}
		for (int i = int(0); i < isi; i++) {
			this.FromCa(0, caM, caP, caD); // just decay to 0
		}
	}

};

#include "slrand.hlsl"

// axon.Time contains all the timing state and parameter information for running a model.
// Can also include other relevant state context, e.g., Testing vs. Training modes.
struct Time {

	// phase counter: typicaly 0-1 for minus-plus but can be more phases for other algorithms
	int Phase;

	// true if this is the plus phase, when the outcome / bursting is occurring, driving positive learning -- else minus phase
	int PlusPhase;

	// cycle within current phase -- minus or plus
	int PhaseCycle;

	// cycle counter: number of iterations of activation updating (settling) on the current state -- this counts time sequentially until reset with NewState
	int Cycle;

	// total cycle count -- this increments continuously from whenever it was last reset -- typically this is number of milliseconds in simulation time
	int CycleTot;

	// accumulated amount of time the network has been running, in simulation-time (not real world time), in seconds
	float Time;

	// if true, the model is being run in a testing mode, so no weight changes or other associated computations are needed.  this flag should only affect learning-related behavior
	int Testing;

	// amount of time to increment per cycle
	float TimePerCyc;

	// random counter
	RandCounter RandCtr;
	void Reset() { this.Phase = 0;; this.PlusPhase = 0;; this.PhaseCycle = 0;; this.Cycle = 0;; this.CycleTot = 0;; this.Time = 0;; this.Testing = 0;; if (this.TimePerCyc == 0) {
		this.TimePerCyc = 0.001;
	}; this.RandCtr.Reset(); }

// Pass the evaluation model associated with this new state --
// if !Train then testing will be set to true.
	void NewState() {
		this.Phase = 0;
		this.PlusPhase = 0;
		this.PhaseCycle = 0;
		this.Cycle = 0;
		// tm.Testing = mode != "Train"
	}

	void NewPhase(bool plusPhase) {
		this.PhaseCycle = 0;
		this.PlusPhase = int(plusPhase);
	}

	void CycleInc() {
		this.PhaseCycle++;
		this.Cycle++;
		this.CycleTot++;
		this.Time += this.TimePerCyc;
	}

};

// Defaults sets default values

// NeuronFlags are bit-flags encoding relevant binary state for neurons
typedef int NeuronFlags;

// The neuron flags

// NeuronOff flag indicates that this neuron has been turned off (i.e., lesioned)
static const NeuronFlags NeuronOff = 1;

// NeuronHasExt means the neuron has external input in its Ext field
static const NeuronFlags NeuronHasExt = 1 << 2;

// NeuronHasTarg means the neuron has external target input in its Target field
static const NeuronFlags NeuronHasTarg = 1 << 3;

// NeuronHasCmpr means the neuron has external comparison input in its Target field -- used for computing
// comparison statistics but does not drive neural activity ever
static const NeuronFlags NeuronHasCmpr = 1 << 4;

// axon.Neuron holds all of the neuron (unit) level variables.
// This is the most basic version, without any optional features.
// All variables accessible via Unit interface must be float
// and start at the top, in contiguous order
struct Neuron {

	// bit flags for binary state variables
	NeuronFlags Flags;

	// index of the layer that this neuron belongs to -- needed for neuron-level parallel code.
	uint LayIndex;

	// index of the sub-level inhibitory pool that this neuron is in (only for 4D shapes, the pool (unit-group / hypercolumn) structure level) -- indicies start at 1 -- 0 is layer-level pool (is 0 if no sub-pools).
	int SubPool;

	// whether neuron has spiked or not on this cycle (0 or 1)
	float Spike;

	// 1 if neuron has spiked within the last 10 cycles (msecs), corresponding to a nominal max spiking rate of 100 Hz, 0 otherwise -- useful for visualization and computing activity levels in terms of average spiked levels.
	float Spiked;

	// rate-coded activation value reflecting instantaneous estimated rate of spiking, based on 1 / ISIAvg.  This drives feedback inhibition in the FFFB function (todo: this will change when better inhibition is implemented), and is integrated over time for ActInt which is then used for performance statistics and layer average activations, etc.  Should not be used for learning or other computations.
	float Act;

	// integrated running-average activation value computed from Act to produce a longer-term integrated value reflecting the overall activation state across a reasonable time scale to reflect overall response of network to current input state -- this is copied to ActM and ActP at the ends of the minus and plus phases, respectively, and used in computing performance-level statistics (which are typically based on ActM).  Should not be used for learning or other computations.
	float ActInt;

	// ActInt activation state at end of third quarter, representing the posterior-cortical minus phase activation -- used for statistics and monitoring network performance. Should not be used for learning or other computations.
	float ActM;

	// ActInt activation state at end of fourth quarter, representing the posterior-cortical plus_phase activation -- used for statistics and monitoring network performance.  Should not be used for learning or other computations.
	float ActP;

	// external input: drives activation of unit from outside influences (e.g., sensory input)
	float Ext;

	// target value: drives learning to produce this activation value
	float Target;

	// time-integrated total excitatory synaptic conductance, with an instantaneous rise time from each spike (in GeRaw) and exponential decay with Dt.GeTau, aggregated over projections -- does *not* include Gbar.E
	float GeSyn;

	// total excitatory conductance, including all forms of excitation (e.g., NMDA) -- does *not* include Gbar.E
	float Ge;

	// time-integrated total inhibitory synaptic conductance, with an instantaneous rise time from each spike (in GiRaw) and exponential decay with Dt.GiTau, aggregated over projections -- does *not* include Gbar.I.  This is added with computed FFFB inhibition to get the full inhibition in Gi
	float GiSyn;

	// total inhibitory synaptic conductance -- the net inhibitory input to the neuron -- does *not* include Gbar.I
	float Gi;

	// total potassium conductance, typically reflecting sodium-gated potassium currents involved in adaptation effects -- does *not* include Gbar.K
	float Gk;

	// net current produced by all channels -- drives update of Vm
	float Inet;

	// membrane potential -- integrates Inet current over time
	float Vm;

	// dendritic membrane potential -- has a slower time constant, is not subject to the VmR reset after spiking
	float VmDend;

	// spike-driven calcium trace for synapse-level Ca-driven learning: exponential integration of SpikeG * Spike at SynTau time constant (typically 30).  Synapses integrate send.CaSyn * recv.CaSyn across M, P, D time integrals for the synaptic trace driving credit assignment in learning. Time constant reflects binding time of Glu to NMDA and Ca buffering postsynaptically, and determines time window where pre * post spiking must overlap to drive learning.
	float CaSyn;

	// spike-driven calcium trace used as a neuron-level proxy for synpatic credit assignment factor based on time-integrated spiking: exponential integration of SpikeG * Spike at MTau time constant (typically 5).  Simulates a calmodulin (CaM) like signal at the most abstract level.
	float CaSpkM;

	// cascaded integration of CaSpkM at PTau time constant (typically 40), representing neuron-level purely spiking version of plus, LTP direction of weight change and capturing the function of CaMKII in the Kinase learning rule. Used for specialized learning and computational functions, statistics, instead of Act.
	float CaSpkP;

	// cascaded integration CaSpkP at DTau time constant (typically 40), representing neuron-level purely spiking version of minus, LTD direction of weight change and capturing the function of DAPK1 in the Kinase learning rule. Used for specialized learning and computational functions, statistics, instead of Act.
	float CaSpkD;

	// minus-phase snapshot of the CaSpkP value -- similar to ActM but using a more directly spike-integrated value.
	float CaSpkPM;

	// recv neuron calcium signal used to drive temporal error difference component of standard learning rule, combining NMDA (NmdaCa) and spiking-driven VGCC (VgccCaInt) calcium sources (vs. CaSpk* which only reflects spiking component).  This is integrated into CaM, CaP, CaD, and temporal derivative is CaP - CaD (CaMKII - DAPK1).  This approximates the backprop error derivative on net input, but VGCC component adds a proportion of recv activation delta as well -- a balance of both works best.  The synaptic-level trace multiplier provides the credit assignment factor, reflecting coincident activity and potentially integrated over longer multi-trial timescales.
	float CaLrn;

	// integrated CaLrn at MTau timescale (typically 5), simulating a calmodulin (CaM) like signal, which then drives CaP, CaD for delta signal driving error-driven learning.
	float CaM;

	// cascaded integration of CaM at PTau time constant (typically 40), representing the plus, LTP direction of weight change and capturing the function of CaMKII in the Kinase learning rule.
	float CaP;

	// cascaded integratoin of CaP at DTau time constant (typically 40), representing the minus, LTD direction of weight change and capturing the function of DAPK1 in the Kinase learning rule.
	float CaD;

	// difference between CaP - CaD -- this is the error signal that drives error-driven learning.
	float CaDiff;

	// Ca integrated like CaSpkP but only starting at MacCycStart cycle, to prevent inclusion of carryover spiking from prior theta cycle trial -- the PTau time constant otherwise results in significant carryover.
	float SpkMaxCa;

	// maximum CaSpkP across one theta cycle time window -- used for specialized algorithms that have more phasic behavior within a single trial, e.g., BG Matrix layer gating.  Also useful for visualization of peak activity of neurons.
	float SpkMax;

	// final CaSpkD activation state at end of previous theta cycle.  used for specialized learning mechanisms that operate on delayed sending activations.
	float SpkPrv;

	// the activation state at specific time point within current state processing window (e.g., 50 msec for beta cycle within standard theta cycle), as saved by SpkSt1() function.  Used for example in hippocampus for CA3, CA1 learning
	float SpkSt1;

	// the activation state at specific time point within current state processing window (e.g., 100 msec for beta cycle within standard theta cycle), as saved by SpkSt2() function.  Used for example in hippocampus for CA3, CA1 learning
	float SpkSt2;

	// recv-unit based learning rate multiplier, reflecting the sigmoid derivative computed from the CaSpkD of recv unit, and the normalized difference CaSpkP - CaSpkD / MAX(CaSpkP - CaSpkD).
	float RLRate;

	// average activation (of minus phase activation state) over long time intervals (time constant = Dt.LongAvgTau) -- useful for finding hog units and seeing overall distribution of activation
	float ActAvg;

	// ActAvg as a proportion of overall layer activation -- this is used for synaptic scaling to match TrgAvg activation -- updated at SlowInterval intervals
	float AvgPct;

	// neuron's target average activation as a proportion of overall layer activation, assigned during weight initialization, driving synaptic scaling relative to AvgPct
	float TrgAvg;

	// change in neuron's target average activation as a result of unit-wise error gradient -- acts like a bias weight.  MPI needs to share these across processors.
	float DTrgAvg;

	// AvgPct - TrgAvg -- i.e., the error in overall activity level relative to set point for this neuron, which drives synaptic scaling -- updated at SlowInterval intervals
	float AvgDif;

	// Attentional modulation factor, which can be set by special layers such as the TRC -- multiplies Ge
	float Attn;

	// current inter-spike-interval -- counts up since last spike.  Starts at -1 when initialized.
	float ISI;

	// average inter-spike-interval -- average time interval between spikes, integrated with ISITau rate constant (relatively fast) to capture something close to an instantaneous spiking rate.  Starts at -1 when initialized, and goes to -2 after first spike, and is only valid after the second spike post-initialization.
	float ISIAvg;

	// accumulating poisson probability factor for driving excitatory noise spiking -- multiply times uniform random deviate at each time step, until it gets below the target threshold based on lambda.
	float GeNoiseP;

	// integrated noise excitatory conductance, added into Ge
	float GeNoise;

	// accumulating poisson probability factor for driving inhibitory noise spiking -- multiply times uniform random deviate at each time step, until it gets below the target threshold based on lambda.
	float GiNoiseP;

	// integrated noise inhibotyr conductance, added into Gi
	float GiNoise;

	// time-averaged Ge value over the minus phase -- useful for stats to set strength of connections etc to get neurons into right range of overall excitatory drive
	float GeM;

	// time-averaged GiSyn value over the minus phase -- useful for stats to set strength of connections etc to get neurons into right range of overall excitatory drive
	float GiM;

	// accumulating voltage-gated gating value for the medium time scale AHP
	float MahpN;

	// slowly accumulating calcium value that drives the slow AHP
	float SahpCa;

	// sAHP gating value
	float SahpN;

	// conductance of sodium-gated potassium channel (KNa) medium dynamics (Slick) -- produces accommodation / adaptation of firing
	float GknaMed;

	// conductance of sodium-gated potassium channel (KNa) slow dynamics (Slack) -- produces accommodation / adaptation of firing
	float GknaSlow;

	// integrated NMDA recv synaptic current -- adds GeRaw and decays with time constant
	float GnmdaSyn;

	// net postsynaptic (recv) NMDA conductance, after Mg V-gating and Gbar -- added directly to Ge as it has the same reversal potential
	float Gnmda;

	// learning version of integrated NMDA recv synaptic current -- adds GeRaw and decays with time constant -- drives NmdaCa that then drives CaM for learning
	float GnmdaLrn;

	// NMDA calcium computed from GnmdaLrn, drives learning via CaM
	float NmdaCa;

	// Sender-based number of open NMDA channels based on spiking activity and consequent glutamate release for all sending synapses -- this is the presynaptic component of NMDA activation that can be used for computing Ca levels for learning -- increases by (1-SnmdaI)*(1-SnmdaO) with spiking and decays otherwise
	float SnmdaO;

	// Sender-based inhibitory factor on NMDA as a function of sending (presynaptic) spiking history, capturing the allosteric dynamics from Urakubo et al (2008) model.  Increases to 1 with every spike, and decays back to 0 with its own longer decay rate.
	float SnmdaI;

	// net GABA-B conductance, after Vm gating and Gbar + Gbase -- applies to Gk, not Gi, for GIRK, with .1 reversal potential.
	float GgabaB;

	// GABA-B / GIRK activation -- time-integrated value with rise and decay time constants
	float GABAB;

	// GABA-B / GIRK internal drive variable -- gets the raw activation and decays
	float GABABx;

	// conductance (via Ca) for VGCC voltage gated calcium channels
	float Gvgcc;

	// activation gate of VGCC channels
	float VgccM;

	// inactivation gate of VGCC channels
	float VgccH;

	// instantaneous VGCC calcium flux -- can be driven by spiking or directly from Gvgcc
	float VgccCa;

	// time-integrated VGCC calcium flux -- this is actually what drives learning
	float VgccCaInt;

	// extra excitatory conductance added to Ge -- from Ext input, deep.GeCtxt etc
	float GeExt;

	// raw excitatory conductance (net input) received from senders = current raw spiking drive
	float GeRaw;

	// baseline level of Ge, added to GeRaw, for intrinsic excitability
	float GeBase;

	// raw inhibitory conductance (net input) received from senders  = current raw spiking drive
	float GiRaw;

	// baseline level of Gi, added to GiRaw, for intrinsic excitability
	float GiBase;

	// SST+ somatostatin positive slow spiking inhibition
	float SSGi;

	// amount of SST+ somatostatin positive slow spiking inhibition applied to dendritic Vm (VmDend)
	float SSGiDend;

	// conductance of A-type K potassium channels
	float Gak;
	bool HasFlag(NeuronFlags flag) {
		return (this.Flags & flag) != 0;
	}

	void SetFlag(NeuronFlags flag) {
		this.Flags |= flag;
	}

	void ClearFlag(NeuronFlags flag) {
		this.Flags &=~flag;
	}

	bool IsOff() {
		return this.HasFlag(NeuronOff);
	}

};




//////////////////////////////////////////////////////////////////////////////////////
//  SpikeParams

// SpikeParams contains spiking activation function params.
// Implements a basic thresholded Vm model, and optionally
// the AdEx adaptive exponential function (adapt is KNaAdapt)
struct SpikeParams {

	// threshold value Theta (Q) for firing output activation (.5 is more accurate value based on AdEx biological parameters and normalization
	float Thr;

	// post-spiking membrane potential to reset to, produces refractory effect if lower than VmInit -- 0.3 is apropriate biologically-based value for AdEx (Brette & Gurstner, 2005) parameters.  See also RTau
	float VmR;

	// post-spiking explicit refractory period, in cycles -- prevents Vm updating for this number of cycles post firing -- Vm is reduced in exponential steps over this period according to RTau, being fixed at Tr to VmR exactly
	int Tr;

	// time constant for decaying Vm down to VmR -- at end of Tr it is set to VmR exactly -- this provides a more realistic shape of the post-spiking Vm which is only relevant for more realistic channels that key off of Vm -- does not otherwise affect standard computation
	float RTau;

	// if true, turn on exponential excitatory current that drives Vm rapidly upward for spiking as it gets past its nominal firing threshold (Thr) -- nicely captures the Hodgkin Huxley dynamics of Na and K channels -- uses Brette & Gurstner 2005 AdEx formulation
	int Exp;

	// slope in Vm (2 mV = .02 in normalized units) for extra exponential excitatory current that drives Vm rapidly upward for spiking as it gets past its nominal firing threshold (Thr) -- nicely captures the Hodgkin Huxley dynamics of Na and K channels -- uses Brette & Gurstner 2005 AdEx formulation
	float ExpSlope;

	// membrane potential threshold for actually triggering a spike when using the exponential mechanism
	float ExpThr;

	// for translating spiking interval (rate) into rate-code activation equivalent, what is the maximum firing rate associated with a maximum activation value of 1
	float MaxHz;

	// constant for integrating the spiking interval in estimating spiking rate
	float ISITau;

	// rate = 1 / tau
	float ISIDt;

	// rate = 1 / tau
	float RDt;

	float pad;
// based on time increment (.001 = 1msec default), Act.Dt.Integ
	float ActToISI(float act, float timeInc, float integ) {
		if (act == 0) {
			return 0;
		}
		return (1 / (timeInc * integ * act * this.MaxHz));
	}

	float ActFromISI(float isi, float timeInc, float integ) {
		if (isi <= 0) {
			return 0;
		}
		float maxInt = 1.0 / (timeInc * integ * this.MaxHz); // interval at max hz..
		return maxInt / isi;                                   // normalized
	}

	void AvgFromISI(inout float avg, float isi) {
		if (avg <= 0) {
			avg = isi;
		} else if (isi < 0.8*avg) {
			avg = isi; // if significantly less than we take that
		} else { // integrate on slower
			avg += this.ISIDt * (isi - avg); // running avg updt
		}
	}

};

// hard min

//////////////////////////////////////////////////////////////////////////////////////
//  DendParams

// DendParams are the parameters for updating dendrite-specific dynamics
struct DendParams {

	// dendrite-specific strength multiplier of the exponential spiking drive on Vm -- e.g., .5 makes it half as strong as at the soma (which uses Gbar.L as a strength multiplier per the AdEx standard model)
	float GbarExp;

	// dendrite-specific conductance of Kdr delayed rectifier currents, used to reset membrane potential for dendrite -- applied for Tr msec
	float GbarR;

	// SST+ somatostatin positive slow spiking inhibition level specifically affecting dendritic Vm (VmDend) -- this is important for countering a positive feedback loop from NMDA getting stronger over the course of learning -- also typically requires SubMean = 1 for TrgAvgAct and learning to fully counter this feedback loop.
	float SSGi;

	float pad;
};

//////////////////////////////////////////////////////////////////////////////////////
//  ActInitParams

// ActInitParams are initial values for key network state variables.
// Initialized in InitActs called by InitWts, and provides target values for DecayState.
struct ActInitParams {

	// initial membrane potential -- see Erev.L for the resting potential (typically .3)
	float Vm;

	// initial activation value -- typically 0
	float Act;

	// baseline level of excitatory conductance (net input) -- Ge is initialized to this value, and it is added in as a constant background level of excitatory input -- captures all the other inputs not represented in the model, and intrinsic excitability, etc
	float Ge;

	// baseline level of inhibitory conductance (net input) -- Gi is initialized to this value, and it is added in as a constant background level of inhibitory input -- captures all the other inputs not represented in the model
	float Gi;

	// variance (sigma) of gaussian distribution around baseline Ge values, per unit, to establish variability in intrinsic excitability.  value never goes < 0
	float GeVar;

	// variance (sigma) of gaussian distribution around baseline Gi values, per unit, to establish variability in intrinsic excitability.  value never goes < 0
	float GiVar;

	float pad, pad1;
};

//////////////////////////////////////////////////////////////////////////////////////
//  DecayParams

// DecayParams control the decay of activation state in the DecayState function
// called in NewState when a new state is to be processed.
struct DecayParams {

	// proportion to decay most activation state variables toward initial values at start of every ThetaCycle (except those controlled separately below) -- if 1 it is effectively equivalent to full clear, resetting other derived values.  ISI is reset every AlphaCycle to get a fresh sample of activations (doesn't affect direct computation -- only readout).
	float Act;

	// proportion to decay long-lasting conductances, NMDA and GABA, and also the dendritic membrane potential -- when using random stimulus order, it is important to decay this significantly to allow a fresh start -- but set Act to 0 to enable ongoing activity to keep neurons in their sensitive regime.
	float Glong;

	// decay of afterhyperpolarization currents, including mAHP, sAHP, and KNa -- has a separate decay because often useful to have this not decay at all even if decay is on.
	float AHP;

	float pad;
};

//////////////////////////////////////////////////////////////////////////////////////
//  DtParams

// DtParams are time and rate constants for temporal derivatives in Axon (Vm, G)
struct DtParams {

	// overall rate constant for numerical integration, for all equations at the unit level -- all time constants are specified in millisecond units, with one cycle = 1 msec -- if you instead want to make one cycle = 2 msec, you can do this globally by setting this integ value to 2 (etc).  However, stability issues will likely arise if you go too high.  For improved numerical stability, you may even need to reduce this value to 0.5 or possibly even lower (typically however this is not necessary).  MUST also coordinate this with network.time_inc variable to ensure that global network.time reflects simulated time accurately
	float Integ;

	// membrane potential time constant in cycles, which should be milliseconds typically (tau is roughly how long it takes for value to change significantly -- 1.4x the half-life) -- reflects the capacitance of the neuron in principle -- biological default for AdEx spiking model C = 281 pF = 2.81 normalized
	float VmTau;

	// dendritic membrane potential time constant in cycles, which should be milliseconds typically (tau is roughly how long it takes for value to change significantly -- 1.4x the half-life) -- reflects the capacitance of the neuron in principle -- biological default for AdEx spiking model C = 281 pF = 2.81 normalized
	float VmDendTau;

	// number of integration steps to take in computing new Vm value -- this is the one computation that can be most numerically unstable so taking multiple steps with proportionally smaller dt is beneficial
	int VmSteps;

	// time constant for decay of excitatory AMPA receptor conductance.
	float GeTau;

	// time constant for decay of inhibitory GABAa receptor conductance.
	float GiTau;

	// time constant for integrating values over timescale of an individual input state (e.g., roughly 200 msec -- theta cycle), used in computing ActInt, and for GeM from Ge -- this is used for scoring performance, not for learning, in cycles, which should be milliseconds typically (tau is roughly how long it takes for value to change significantly -- 1.4x the half-life),
	float IntTau;

	// time constant for integrating slower long-time-scale averages, such as nrn.ActAvg, Pool.ActsMAvg, ActsPAvg -- computed in NewState when a new input state is present (i.e., not msec but in units of a theta cycle) (tau is roughly how long it takes for value to change significantly) -- set lower for smaller models
	float LongAvgTau;

	// cycle to start updating the SpkMaxCa, SpkMax values within a theta cycle -- early cycles often reflect prior state
	int MaxCycStart;

	// nominal rate = Integ / tau
	float VmDt;

	// nominal rate = Integ / tau
	float VmDendDt;

	// 1 / VmSteps
	float DtStep;

	// rate = Integ / tau
	float GeDt;

	// rate = Integ / tau
	float GiDt;

	// rate = Integ / tau
	float IntDt;

	// rate = 1 / tau
	float LongAvgDt;
	float GeSynFromRaw(float geSyn, float geRaw) {
		return geSyn + geRaw - this.GeDt*geSyn;
	}

// receiving a steady increment of GeRaw every time step = raw * GeTau.
// dSyn = Raw - dt*Syn; solve for dSyn = 0 to get steady state:
// dt*Syn = Raw; Syn = Raw / dt = Raw * Tau
	float GeSynFromRawSteady(float geRaw) {
		return geRaw * this.GeTau;
	}

	float GiSynFromRaw(float giSyn, float giRaw) {
		return giSyn + giRaw - this.GiDt*giSyn;
	}

// receiving a steady increment of GiRaw every time step = raw * GiTau.
// dSyn = Raw - dt*Syn; solve for dSyn = 0 to get steady state:
// dt*Syn = Raw; Syn = Raw / dt = Raw * Tau
	float GiSynFromRawSteady(float giRaw) {
		return giRaw * this.GiTau;
	}

	void AvgVarUpdate(inout float avg, inout float vr, float val) {
		if (avg == 0) { // first time -- set
			avg = val;
			vr = 0;
		} else {
			float del = val - avg;
			float incr = this.LongAvgDt * del;
			avg += incr;
			// following is magic exponentially-weighted incremental variance formula
			// derived by Finch, 2009: Incremental calculation of weighted mean and variance
			if (vr == 0) {
				vr = 2 * (1 - this.LongAvgDt) * del * incr;
			} else {
				vr = (1 - this.LongAvgDt) * (vr + del*incr);
			}
		}
	}

};

//////////////////////////////////////////////////////////////////////////////////////
//  Noise

// SpikeNoiseParams parameterizes background spiking activity impinging on the neuron,
// simulated using a poisson spiking process.
struct SpikeNoiseParams {

	// add noise simulating background spiking levels
	int On;

	// mean frequency of excitatory spikes -- typically 50Hz but multiple inputs increase rate -- poisson lambda parameter, also the variance
	float GeHz;

	// excitatory conductance per spike -- .001 has minimal impact, .01 can be strong, and .15 is needed to influence timing of clamped inputs
	float Ge;

	// mean frequency of inhibitory spikes -- typically 100Hz fast spiking but multiple inputs increase rate -- poisson lambda parameter, also the variance
	float GiHz;

	// excitatory conductance per spike -- .001 has minimal impact, .01 can be strong, and .15 is needed to influence timing of clamped inputs
	float Gi;

	// Exp(-Interval) which is the threshold for GeNoiseP as it is updated
	float GeExpInt;

	// Exp(-Interval) which is the threshold for GiNoiseP as it is updated
	float GiExpInt;

	float pad;
// and returns Ge from spiking if a spike is triggered
	float PGe(inout float p, int ni, inout uint2 randctr) {
		p *= RandFloat(randctr, uint(ni));
		if (p <= this.GeExpInt) {
			p = 1;
			return this.Ge;
		}
		return 0;
	}

// and returns Gi from spiking if a spike is triggered
	float PGi(inout float p, int ni, inout uint2 randctr) {
		p *= RandFloat(randctr, uint(ni));
		if (p <= this.GiExpInt) {
			p = 1;
			return this.Gi;
		}
		return 0;
	}

};

//////////////////////////////////////////////////////////////////////////////////////
//  ClampParams

// ClampParams specify how external inputs drive excitatory conductances
// (like a current clamp) -- either adds or overwrites existing conductances.
// Noise is added in either case.
struct ClampParams {

	// amount of Ge driven for clamping -- generally use 0.8 for Target layers, 1.5 for Input layers
	float Ge;

	//
	int Add;

	// threshold on neuron Act activity to count as active for computing error relative to target in PctErr method
	float ErrThr;

	float pad;
};

//////////////////////////////////////////////////////////////////////////////////////
//  AttnParams

// AttnParams determine how the Attn modulates Ge
struct AttnParams {

	// is attentional modulation active?
	int On;

	// minimum act multiplier if attention is 0
	float Min;

	float pad, pad1;
	float ModValue(float val, float attn) {
		if (val < 0) {
			val = 0;
		}
		if (this.On==0) {
			return val;
		}
		return val * (this.Min + (1-this.Min)*attn);
	}

};

//////////////////////////////////////////////////////////////////////////////////////
//  SynComParams

// SynComParams are synaptic communication parameters: delay and probability of failure
struct SynComParams {

	// additional synaptic delay for inputs arriving at this projection -- IMPORTANT: if you change this, you must call InitWts() on Network!  Delay = 0 means a spike reaches receivers in the next Cycle, which is the minimum time.  Biologically, subtract 1 from synaptic delay values to set corresponding Delay value.
	int Delay;

	// probability of synaptic transmission failure -- if > 0, then weights are turned off at random as a function of PFail (times 1-SWt if PFailSwt)
	float PFail;

	// if true, then probability of failure is inversely proportional to SWt structural / slow weight value (i.e., multiply PFail * (1-SWt)))
	int PFailSWt;

	float pad;
	float WtFailP(float swt) {
		if (this.PFailSWt==0) {
			return this.PFail;
		}
		return this.PFail * (1 - swt);
	}

};

// 0.5 works?

// axon.ActParams contains all the activation computation params and functions
// for basic Axon, at the neuron level .
// This is included in axon.Layer to drive the computation.
struct ActParams {

	// Spiking function parameters
	SpikeParams Spike;

	// dendrite-specific parameters
	DendParams Dend;

	// initial values for key network state variables -- initialized in InitActs called by InitWts, and provides target values for DecayState
	ActInitParams Init;

	// amount to decay between AlphaCycles, simulating passage of time and effects of saccades etc, especially important for environments with random temporal structure (e.g., most standard neural net training corpora)
	DecayParams Decay;

	// time and rate constants for temporal derivatives / updating of activation state
	DtParams Dt;

	// maximal conductances levels for channels
	Chans Gbar;

	// reversal potentials for each channel
	Chans Erev;

	// how external inputs drive neural activations
	ClampParams Clamp;

	// how, where, when, and how much noise to add
	SpikeNoiseParams Noise;

	// range for Vm membrane potential -- -- important to keep just at extreme range of reversal potentials to prevent numerical instability
	F32 VmRange;

	// M-type medium time-scale afterhyperpolarization mAHP current -- this is the primary form of adaptation on the time scale of multiple sequences of spikes
	MahpParams Mahp;

	// slow time-scale afterhyperpolarization sAHP current -- integrates SpkCaD at theta cycle intervals and produces a hard cutoff on sustained activity for any neuron
	SahpParams Sahp;

	// sodium-gated potassium channel adaptation parameters -- activates a leak-like current as a function of neural activity (firing = Na influx) at two different time-scales (Slick = medium, Slack = slow)
	KNaMedSlow KNa;

	// NMDA channel parameters used in computing Gnmda conductance for bistability, and postsynaptic calcium flux used in learning.  Note that Learn.Snmda has distinct parameters used in computing sending NMDA parameters used in learning.
	NMDAParams NMDA;

	// GABA-B / GIRK channel parameters
	GABABParams GABAB;

	// voltage gated calcium channels -- provide a key additional source of Ca for learning and positive-feedback loop upstate for active neurons
	VGCCParams VGCC;

	// A-type potassium (K) channel that is particularly important for limiting the runaway excitation from VGCC channels
	AKsParams AK;

	// Attentional modulation parameters: how Attn modulates Ge
	AttnParams Attn;
// in proportion to given decay parameter.  Special case values
// such as Glong and KNa are also decayed with their
// separately parameterized values.
// Called with ac.Decay.Act by Layer during NewState
	void DecayState(inout Neuron nrn, float decay, float glong) {
		// always reset these -- otherwise get insanely large values that take forever to update
		nrn.ISI = -1;
		nrn.ISIAvg = -1;
		nrn.ActInt = this.Init.Act; // start fresh

		if (decay > 0) { // no-op for most, but not all..
			nrn.Spike = 0;
			nrn.Spiked = 0;
			nrn.Act -= decay * (nrn.Act - this.Init.Act);
			nrn.ActInt -= decay * (nrn.ActInt - this.Init.Act);
			nrn.GeSyn -= decay * (nrn.GeSyn - nrn.GeBase);
			nrn.Ge -= decay * (nrn.Ge - nrn.GeBase);
			nrn.Gi -= decay * (nrn.Gi - nrn.GiBase);
			nrn.Gk -= decay * nrn.Gk;

			nrn.Vm -= decay * (nrn.Vm - this.Init.Vm);

			nrn.GeNoise -= decay * nrn.GeNoise;
			nrn.GiNoise -= decay * nrn.GiNoise;

			nrn.GiSyn -= decay * nrn.GiSyn;
		}

		nrn.VmDend -= glong * (nrn.VmDend - this.Init.Vm);

		nrn.MahpN -= this.Decay.AHP * nrn.MahpN;
		nrn.SahpCa -= this.Decay.AHP * nrn.SahpCa;
		nrn.SahpN -= this.Decay.AHP * nrn.SahpN;
		nrn.GknaMed -= this.Decay.AHP * nrn.GknaMed;
		nrn.GknaSlow -= this.Decay.AHP * nrn.GknaSlow;

		nrn.GgabaB -= glong * nrn.GgabaB;
		nrn.GABAB -= glong * nrn.GABAB;
		nrn.GABABx -= glong * nrn.GABABx;

		nrn.Gvgcc -= glong * nrn.Gvgcc;
		nrn.VgccM -= glong * nrn.VgccM;
		nrn.VgccH -= glong * nrn.VgccH;
		nrn.Gak -= glong * nrn.Gak;

		nrn.GnmdaSyn -= glong * nrn.GnmdaSyn;
		nrn.Gnmda -= glong * nrn.Gnmda;

		// learning-based NMDA, Ca values decayed in Learn.DecayNeurCa

		nrn.Inet = 0;
		nrn.GeRaw = 0;
		nrn.GiRaw = 0;
		nrn.SSGi = 0;
		nrn.SSGiDend = 0;
		nrn.GeExt = 0;
	}

// automatically called (DecayState is used instead)
	void InitActs(inout Neuron nrn) {
		nrn.Spike = 0;
		nrn.Spiked = 0;
		nrn.ISI = -1;
		nrn.ISIAvg = -1;
		nrn.Act = this.Init.Act;
		nrn.ActInt = this.Init.Act;
		nrn.GeBase = 0;
		nrn.GiBase = 0;
		nrn.GeSyn = nrn.GeBase;
		nrn.Ge = nrn.GeBase;
		nrn.Gi = nrn.GiBase;
		nrn.Gk = 0;
		nrn.Inet = 0;
		nrn.Vm = this.Init.Vm;
		nrn.VmDend = this.Init.Vm;
		nrn.Target = 0;
		nrn.Ext = 0;

		nrn.SpkMaxCa = 0;
		nrn.SpkMax = 0;
		nrn.Attn = 1;
		nrn.RLRate = 1;

		nrn.GeNoiseP = 1;
		nrn.GeNoise = 0;
		nrn.GiNoiseP = 1;
		nrn.GiNoise = 0;

		nrn.GiSyn = 0;

		nrn.MahpN = 0;
		nrn.SahpCa = 0;
		nrn.SahpN = 0;
		nrn.GknaMed = 0;
		nrn.GknaSlow = 0;

		nrn.GnmdaSyn = 0;
		nrn.Gnmda = 0;
		nrn.SnmdaO = 0;
		nrn.SnmdaI = 0;

		nrn.GgabaB = 0;
		nrn.GABAB = 0;
		nrn.GABABx = 0;

		nrn.Gvgcc = 0;
		nrn.VgccM = 0;
		nrn.VgccH = 0;
		nrn.Gak = 0;

		nrn.GeRaw = 0;
		nrn.GiRaw = 0;
		nrn.SSGi = 0;
		nrn.SSGiDend = 0;
		nrn.GeExt = 0;

		this.InitLongActs(nrn);
	}

// (SpkPrv, SpkSt*, ActM, ActP, GeM)
// Called from InitActs, which is called from InitWts,
// but otherwise not automatically called
// (DecayState is used instead)
	void InitLongActs(inout Neuron nrn) {
		nrn.SpkPrv = 0;
		nrn.SpkSt1 = 0;
		nrn.SpkSt2 = 0;
		nrn.ActM = 0;
		nrn.ActP = 0;
		nrn.GeM = 0;
	}

// total Ge (GeRaw + Ext) and current Vm, Spiking
	void NMDAFromRaw(inout Neuron nrn, float geTot) {
		if (geTot < 0) {
			geTot = 0;
		}
		nrn.GnmdaSyn = this.NMDA.NMDASyn(nrn.GnmdaSyn, geTot);
		nrn.Gnmda = this.NMDA.Gnmda(nrn.GnmdaSyn, nrn.VmDend);
		// note: nrn.NmdaCa computed via Learn.LrnNMDA in learn.go, CaM method
	}

// from VmDend
	void GvgccFromVm(inout Neuron nrn) {
		nrn.Gvgcc = this.VGCC.Gvgcc(nrn.VmDend, nrn.VgccM, nrn.VgccH);
		float dm, dh;
		this.VGCC.DMHFromV(nrn.VmDend, nrn.VgccM, nrn.VgccH, dm, dh);
		nrn.VgccM += dm;
		nrn.VgccH += dh;
		nrn.VgccCa = this.VGCC.CaFromG(nrn.VmDend, nrn.Gvgcc, nrn.VgccCa); // note: may be overwritten!
	}

	void GkFromVm(inout Neuron nrn) {
		float dn = this.Mahp.DNFromV(nrn.Vm, nrn.MahpN);
		nrn.MahpN += dn;
		nrn.Gak = this.AK.Gak(nrn.VmDend);
		nrn.Gk = nrn.Gak + this.Mahp.GmAHP(nrn.MahpN) + this.Sahp.GsAHP(nrn.SahpN);
		if (this.KNa.On==1) {
			this.KNa.GcFromSpike(nrn.GknaMed, nrn.GknaSlow, nrn.Spike > .5);
			nrn.Gk += nrn.GknaMed + nrn.GknaSlow;
		}
	}

// geExt is extra conductance to add to the final Ge value
	void GeFromSyn(int ni, inout Neuron nrn, float geSyn, float geExt, inout uint2 randctr) {
		nrn.GeExt = 0;
		if (this.Clamp.Add==1 && nrn.HasFlag(NeuronHasExt)) {
			nrn.GeExt = nrn.Ext * this.Clamp.Ge;
			geSyn += nrn.GeExt;
		}
		geSyn = this.Attn.ModValue(geSyn, nrn.Attn);

		if (this.Clamp.Add==1 && nrn.HasFlag(NeuronHasExt)) {
			geSyn = nrn.Ext * this.Clamp.Ge;
			nrn.GeExt = geSyn;
			geExt = 0; // no extra in this case
		}

		nrn.Ge = geSyn + geExt;
		if (nrn.Ge < 0) {
			nrn.Ge = 0;
		}
		this.GeNoise(ni, nrn, randctr);
	}

	void GeNoise(int ni, inout Neuron nrn, inout uint2 randctr) {
		if ((0 == this.Noise.On) || this.Noise.Ge == 0) {
			return;
		}
		float ge = this.Noise.PGe(nrn.GeNoiseP, ni, randctr);
		nrn.GeNoise = this.Dt.GeSynFromRaw(nrn.GeNoise, ge);
		nrn.Ge += nrn.GeNoise;
	}

	void GiNoise(int ni, inout Neuron nrn, inout uint2 randctr) {
		if ((0 == this.Noise.On) || this.Noise.Gi == 0) {
			return;
		}
		float gi = this.Noise.PGi(nrn.GiNoiseP, ni, randctr);
		// fmt.Printf("rc: %v\n", *randctr)
		nrn.GiNoise = this.Dt.GiSynFromRaw(nrn.GiNoise, gi);
	}

// (can add other terms to geRaw prior to calling this)
	float GiFromSyn(int ni, inout Neuron nrn, float giSyn, inout uint2 randctr) {
		this.GiNoise(ni, nrn, randctr);
		if (giSyn < 0) { // negative inhib G doesn't make any sense
			giSyn = 0;
		}
		return giSyn;
	}

	float InetFromG(float vm, float ge, float gl, float gi, float gk) {
		float inet = ge*(this.Erev.E-vm) + gl*this.Gbar.L*(this.Erev.L-vm) + gi*(this.Erev.I-vm) + gk*(this.Erev.K-vm);
		if (inet > this.Dt.VmTau) {
			inet = this.Dt.VmTau;
		} else if (inet < -this.Dt.VmTau) {
			inet = -this.Dt.VmTau;
		}
		return inet;
	}

	float VmFromInet(float vm, float dt, float inet) {
		return this.VmRange.ClipValue(vm + dt*inet);
	}

// Returns the new Vm and inet values.
	void VmInteg(float vm, float dt, float ge, float gl, float gi, float gk, inout float nvm, inout float inet) {
		dt *= this.Dt.DtStep;
		nvm = vm;
		for (int i = int(0); i < this.Dt.VmSteps; i++) {
			inet = this.InetFromG(nvm, ge, gl, gi, gk);
			nvm = this.VmFromInet(nvm, dt, inet);
		}
	}

	void VmFromG(inout Neuron nrn) {
		bool updtVm = true;
		// note: nrn.ISI has NOT yet been updated at this point: 0 right after spike, etc
		// so it takes a full 3 time steps after spiking for Tr period
		if (this.Spike.Tr > 0 && nrn.ISI >= 0 && nrn.ISI < float(this.Spike.Tr)) {
			updtVm = false; // don't update the spiking vm during refract
		}

		float ge = nrn.Ge * this.Gbar.E;
		float gi = nrn.Gi * this.Gbar.I;
		float gk = nrn.Gk * this.Gbar.K;
		float nvm, inet, exVm, expi;
		if (updtVm) {
			this.VmInteg(nrn.Vm, this.Dt.VmDt, ge, 1, gi, gk, nvm, inet);
			if (updtVm && (1 == this.Spike.Exp)) { // add spike current if relevant
				exVm = 0.5 * (nvm + nrn.Vm); // midpoint for this
				expi = this.Gbar.L * this.Spike.ExpSlope *
					FastExp((exVm-this.Spike.Thr)/this.Spike.ExpSlope);
				if (expi > this.Dt.VmTau) {
					expi = this.Dt.VmTau;
				}
				inet += expi;
				nvm = this.VmFromInet(nvm, this.Dt.VmDt, expi);
			}
			nrn.Vm = nvm;
			nrn.Inet = inet;
		} else { // decay back to VmR
			float dvm;
			if (int(nrn.ISI) == this.Spike.Tr-1) {
				dvm = (this.Spike.VmR - nrn.Vm);
			} else {
				dvm = this.Spike.RDt * (this.Spike.VmR - nrn.Vm);
			}
			nrn.Vm = nrn.Vm + dvm;
			nrn.Inet = dvm * this.Dt.VmTau;
		}

		{ // always update VmDend
			float glEff = float(1);
			if (!updtVm) {
				glEff += this.Dend.GbarR;
			}
			float giEff = gi + this.Gbar.I*nrn.SSGiDend;
			this.VmInteg(nrn.VmDend, this.Dt.VmDendDt, ge, glEff, giEff, gk, nvm, inet);
			if (updtVm) {
				nvm = this.VmFromInet(nvm, this.Dt.VmDendDt, this.Dend.GbarExp*expi);
			}
			nrn.VmDend = nvm;
		}
	}

	void SpikeFromVm(inout Neuron nrn) {
		float thr;
		if ((1 == this.Spike.Exp)) {
			thr = this.Spike.ExpThr;
		} else {
			thr = this.Spike.Thr;
		}
		if (nrn.Vm >= thr) {
			nrn.Spike = 1;
			if (nrn.ISIAvg == -1) {
				nrn.ISIAvg = -2;
			} else if (nrn.ISI > 0) { // must have spiked to update
				this.Spike.AvgFromISI(nrn.ISIAvg, nrn.ISI+1);
			}
			nrn.ISI = 0;
		} else {
			nrn.Spike = 0;
			if (nrn.ISI >= 0) {
				nrn.ISI += 1;
				if (nrn.ISI < 10) {
					nrn.Spiked = 1;
				} else {
					nrn.Spiked = 0;
				}
			} else {
				nrn.Spiked = 0;
			}
			if (nrn.ISIAvg >= 0 && nrn.ISI > 0 && nrn.ISI > 1.2*nrn.ISIAvg) {
				this.Spike.AvgFromISI(nrn.ISIAvg, nrn.ISI);
			}
		}

		float nwAct = this.Spike.ActFromISI(nrn.ISIAvg, .001, this.Dt.Integ);
		if (nwAct > 1) {
			nwAct = 1;
		}
		nwAct = nrn.Act + this.Dt.VmDt*(nwAct-nrn.Act);
		nrn.Act = nwAct;
	}

};

// E, L, I, K: gbar l = 0.2 > 0.1
// E, L, I, K: K = hyperpolarized -90mv

// .15 now -- was 0.3 best.

// Update must be called after any changes to parameters

///////////////////////////////////////////////////////////////////////
//  Init

///////////////////////////////////////////////////////////////////////
//  Cycle

// CaLrnParams parameterizes the neuron-level calcium signals driving learning:
// CaLrn = NMDA + VGCC Ca sources, where VGCC can be simulated from spiking or
// use the more complex and dynamic VGCC channel directly.
// CaLrn is then integrated in a cascading manner at multiple time scales:
// CaM (as in calmodulin), CaP (ltP, CaMKII, plus phase), CaD (ltD, DAPK1, minus phase).
struct CaLrnParams {

	// denomenator used for normalizing CaLrn, so the max is roughly 1 - 1.5 or so, which works best in terms of previous standard learning rules, and overall learning performance
	float Norm;

	// use spikes to generate VGCC instead of actual VGCC current -- see SpkVGCCa for calcium contribution from each spike
	int SpkVGCC;

	// multiplier on spike for computing Ca contribution to CaLrn in SpkVGCC mode
	float SpkVgccCa;

	// time constant of decay for VgccCa calcium -- it is highly transient around spikes, so decay and diffusion factors are more important than for long-lasting NMDA factor.  VgccCa is integrated separately int VgccCaInt prior to adding into NMDA Ca in CaLrn
	float VgccTau;

	// time constants for integrating CaLrn across M, P and D cascading levels
	CaDtParams Dt;

	// rate = 1 / tau
	float VgccDt;

	// = 1 / Norm
	float NormInv;

	float pad, pad1;
// and performs time-integration of VgccCa
	void VgccCa(inout Neuron nrn) {
		if ((1 == this.SpkVGCC)) {
			nrn.VgccCa = this.SpkVgccCa * nrn.Spike;
		}
		nrn.VgccCaInt += nrn.VgccCa - this.VgccDt*nrn.VgccCaInt; // Dt only affects decay, not rise time
	}

// it first calls VgccCa to update the spike-driven version of that variable, and
// perform its time-integration.
	void CaLrn(inout Neuron nrn) {
		this.VgccCa(nrn);
		nrn.CaLrn = this.NormInv * (nrn.NmdaCa + nrn.VgccCaInt);
		nrn.CaM += this.Dt.MDt * (nrn.CaLrn - nrn.CaM);
		nrn.CaP += this.Dt.PDt * (nrn.CaM - nrn.CaP);
		nrn.CaD += this.Dt.DDt * (nrn.CaP - nrn.CaD);
		nrn.CaDiff = nrn.CaP - nrn.CaD;
	}

};

// CaSpkParams parameterizes the neuron-level spike-driven calcium
// signals, starting with CaSyn that is integrated at the neuron level
// and drives synapse-level, pre * post Ca integration, which provides the Tr
// trace that multiplies error signals, and drives learning directly for Target layers.
// CaSpk* values are integrated separately at the Neuron level and used for UpdateThr
// and RLRate as a proxy for the activation (spiking) based learning signal.
struct CaSpkParams {

	// gain multiplier on spike for computing CaSpk: increasing this directly affects the magnitude of the trace values, learning rate in Target layers, and other factors that depend on CaSpk values: RLRate, UpdateThr.  Prjn.KinaseCa.SpikeG provides an additional gain factor specific to the synapse-level trace factors, without affecting neuron-level CaSpk values.  Larger networks require higher gain factors at the neuron level -- 12, vs 8 for smaller.
	float SpikeG;

	// time constant for integrating spike-driven calcium trace at sender and recv neurons, CaSyn, which then drives synapse-level integration of the joint pre * post synapse-level activity, in cycles (msec)
	float SynTau;

	// rate = 1 / tau
	float SynDt;

	// Ca gain factor for SynSpkCa learning rule, to compensate for the effect of SynTau, which increases Ca as it gets larger.  is 1 for SynTau = 30 -- todo: eliminate this at some point!
	float SynSpkG;

	// time constants for integrating CaSpk across M, P and D cascading levels -- these are typically the same as in CaLrn and Prjn level for synaptic integration, except for the M factor.
	CaDtParams Dt;
	void CaFromSpike(inout Neuron nrn) {
		float nsp = this.SpikeG * nrn.Spike;
		nrn.CaSyn += this.SynDt * (nsp - nrn.CaSyn);
		nrn.CaSpkM += this.Dt.MDt * (nsp - nrn.CaSpkM);
		nrn.CaSpkP += this.Dt.PDt * (nrn.CaSpkM - nrn.CaSpkP);
		nrn.CaSpkD += this.Dt.DDt * (nrn.CaSpkP - nrn.CaSpkD);
	}

};

//////////////////////////////////////////////////////////////////////////////////////
//  TrgAvgActParams

// TrgAvgActParams govern the target and actual long-term average activity in neurons.
// Target value is adapted by neuron-wise error and difference in actual vs. target.
// drives synaptic scaling at a slow timescale (Network.SlowInterval).
struct TrgAvgActParams {

	// whether to use target average activity mechanism to scale synaptic weights
	int On;

	// learning rate for adjustments to Trg value based on unit-level error signal.  Population TrgAvg values are renormalized to fixed overall average in TrgRange. Generally, deviating from the default doesn't make much difference.
	float ErrLRate;

	// rate parameter for how much to scale synaptic weights in proportion to the AvgDif between target and actual proportion activity -- this determines the effective strength of the constraint, and larger models may need more than the weaker default value.
	float SynScaleRate;

	// amount of mean trg change to subtract -- 1 = full zero sum.  1 works best in general -- but in some cases it may be better to start with 0 and then increase using network SetSubMean method at a later point.
	float SubMean;

	// permute the order of TrgAvg values within layer -- otherwise they are just assigned in order from highest to lowest for easy visualization -- generally must be true if any topographic weights are being used
	int Permute;

	// use pool-level target values if pool-level inhibition and 4D pooled layers are present -- if pool sizes are relatively small, then may not be useful to distribute targets just within pool
	int Pool;

	float pad, pad1;

	// range of target normalized average activations -- individual neurons are assigned values within this range to TrgAvg, and clamped within this range.
	F32 TrgRange;
};

// 1 in general beneficial

//////////////////////////////////////////////////////////////////////////////////////
//  RLRateParams

// RLRateParams are recv neuron learning rate modulation parameters.
// Has two factors: the derivative of the sigmoid based on CaSpkD
// activity levels, and based on the phase-wise differences in activity (Diff).
struct RLRateParams {

	// use learning rate modulation
	int On;

	// minimum learning rate multiplier for sigmoidal act (1-act) factor -- prevents lrate from going too low for extreme values.  Set to 1 to disable Sigmoid derivative factor, which is default for Target layers.
	float SigmoidMin;

	// modulate learning rate as a function of plus - minus differences
	int Diff;

	// threshold on Max(CaSpkP, CaSpkD) below which Min lrate applies -- must be > 0 to prevent div by zero
	float SpkThr;

	// threshold on recv neuron error delta, i.e., |CaSpkP - CaSpkD| below which lrate is at Min value
	float DiffThr;

	// for Diff component, minimum learning rate value when below ActDiffThr
	float Min;

	float pad, pad1;
// factor as a function of spiking activity, with mid-range values having
// full learning and extreme values a reduced learning rate:
// deriv = act * (1 - act)
// The activity should be CaSpkP and the layer maximum is used
// to normalize that to a 0-1 range.
	float RLRateSigDeriv(float act, float laymax) {
		if ((0 == this.On) || laymax == 0) {
			return 1.0;
		}
		float ca = act / laymax;
		float lr = 4.0 * ca * (1 - ca); // .5 * .5 = .25 = peak
		if (lr < this.SigmoidMin) {
			lr = this.SigmoidMin;
		}
		return lr;
	}

// CaSpkP and CaSpkD values
	float RLRateDiff(float scap, float scad) {
		if ((0 == this.On) || (0 == this.Diff)) {
			return 1.0;
		}
		float mx = max(scap, scad);
		if (mx > this.SpkThr) { // avoid div by 0
			float dif = abs(scap - scad);
			if (dif < this.DiffThr) {
				return this.Min;
			}
			return (dif / mx);
		}
		return this.Min;
	}

};

// axon.LearnNeurParams manages learning-related parameters at the neuron-level.
// This is mainly the running average activations that drive learning
struct LearnNeurParams {

	// parameterizes the neuron-level calcium signals driving learning: CaLrn = NMDA + VGCC Ca sources, where VGCC can be simulated from spiking or use the more complex and dynamic VGCC channel directly.  CaLrn is then integrated in a cascading manner at multiple time scales: CaM (as in calmodulin), CaP (ltP, CaMKII, plus phase), CaD (ltD, DAPK1, minus phase).
	CaLrnParams CaLrn;

	// parameterizes the neuron-level spike-driven calcium signals, starting with CaSyn that is integrated at the neuron level, and drives synapse-level, pre * post Ca integration, which provides the Tr trace that multiplies error signals, and drives learning directly for Target layers. CaSpk* values are integrated separately at the Neuron level and used for UpdateThr and RLRate as a proxy for the activation (spiking) based learning signal.
	CaSpkParams CaSpk;

	// NMDA channel parameters used for learning, vs. the ones driving activation -- allows exploration of learning parameters independent of their effects on active maintenance contributions of NMDA, and may be supported by different receptor subtypes
	NMDAParams LrnNMDA;

	// synaptic scaling parameters for regulating overall average activity compared to neuron's own target level
	TrgAvgActParams TrgAvgAct;

	// recv neuron learning rate modulation params -- an additional error-based modulation of learning for receiver side: RLRate = |SpkCaP - SpkCaD| / Max(SpkCaP, SpkCaD)
	RLRateParams RLRate;
// Called by InitWts (at start of learning).
	void InitNeurCa(inout Neuron nrn) {
		nrn.GnmdaLrn = 0;
		nrn.NmdaCa = 0;
		nrn.SnmdaO = 0;
		nrn.SnmdaI = 0;

		nrn.VgccCa = 0;
		nrn.VgccCaInt = 0;

		nrn.CaLrn = 0;

		nrn.CaSyn = 0;
		nrn.CaSpkM = 0;
		nrn.CaSpkP = 0;
		nrn.CaSpkD = 0;
		nrn.CaSpkPM = 0;

		nrn.CaM = 0;
		nrn.CaP = 0;
		nrn.CaD = 0;
		nrn.CaDiff = 0;
	}

// by given factor.  Note: this is NOT called by default and is generally
// not useful, causing variability in these learning factors as a function
// of the decay parameter that then has impacts on learning rates etc.
// It is only here for reference or optional testing.
	void DecayCaLrnSpk(inout Neuron nrn, float decay) {
		nrn.GnmdaLrn -= decay * nrn.GnmdaLrn;
		nrn.NmdaCa -= decay * nrn.NmdaCa;
		nrn.SnmdaO -= decay * nrn.SnmdaO;
		nrn.SnmdaI -= decay * nrn.SnmdaI;

		nrn.VgccCa -= decay * nrn.VgccCa;
		nrn.VgccCaInt -= decay * nrn.VgccCaInt;

		nrn.CaLrn -= decay * nrn.CaLrn;

		nrn.CaSyn -= decay * nrn.CaSyn;
		nrn.CaSpkM -= decay * nrn.CaSpkM;
		nrn.CaSpkP -= decay * nrn.CaSpkP;
		nrn.CaSpkD -= decay * nrn.CaSpkD;

		nrn.CaM -= decay * nrn.CaM;
		nrn.CaP -= decay * nrn.CaP;
		nrn.CaD -= decay * nrn.CaD;
	}

// based on GeTot = GeRaw + external ge conductance.  These are the variables
// that drive learning -- can be the same as activation but also can be different
// for testing learning Ca effects independent of activation effects.
	void LrnNMDAFromRaw(inout Neuron nrn, float geTot) {
		if (geTot < 0) {
			geTot = 0;
		}
		nrn.GnmdaLrn = this.LrnNMDA.NMDASyn(nrn.GnmdaLrn, geTot);
		float gnmda = this.LrnNMDA.Gnmda(nrn.GnmdaLrn, nrn.VmDend);
		nrn.NmdaCa = gnmda * this.LrnNMDA.CaFromV(nrn.VmDend);
		this.LrnNMDA.SnmdaFromSpike(nrn.Spike, nrn.SnmdaO, nrn.SnmdaI);
	}

// Computed after new activation for current cycle is updated.
	void CaFromSpike(inout Neuron nrn) {
		this.CaSpk.CaFromSpike(nrn);
		this.CaLrn.CaLrn(nrn);
	}

};

// axon.Layer implements the basic Axon spiking activation function,
// and manages learning in the projections.
struct Layer {

	// Activation parameters and methods for computing activations
	ActParams Act;

	// Learning parameters and methods that operate at the neuron level
	LearnNeurParams Learn;
// and updates GABAB as well
	void GiInteg(int ni, inout Neuron nrn, inout Time ctime) {
		nrn.Gi = nrn.GiSyn + nrn.GiNoise;
		nrn.SSGiDend = this.Act.Dend.SSGi;
		nrn.GABAB = this.Act.GABAB.GFromGX(nrn.GABAB, nrn.GABABx);
		nrn.GABABx = this.Act.GABAB.XFromGiX(nrn.GABABx, nrn.Gi);
		nrn.GgabaB = this.Act.GABAB.GgabaB(nrn.GABAB, nrn.VmDend);
		nrn.Gk += nrn.GgabaB; // Gk was already init
	}

// from the Prjn-level GSyn integrated values.
	void GFromSpikeRaw(int ni, inout Neuron nrn, inout Time ctime) {
		nrn.GeRaw = 0.4;
		nrn.GiRaw = 0;
		nrn.GeSyn = nrn.GeBase;
		nrn.GiSyn = nrn.GiBase;
		nrn.GeSyn = nrn.GeRaw;
	}

// from GeRaw and GeSyn values, including NMDA, VGCC, AMPA, and GABA-A channels.
	void GFromRawSyn(int ni, inout Neuron nrn, inout Time ctime, inout uint2 randctr) {
		this.Act.NMDAFromRaw(nrn, nrn.GeRaw);
		this.Learn.LrnNMDAFromRaw(nrn, nrn.GeRaw);
		this.Act.GvgccFromVm(nrn);
		this.Act.GeFromSyn(ni, nrn, nrn.GeSyn, nrn.Gnmda+nrn.Gvgcc, randctr); // sets nrn.GeExt too
		this.Act.GkFromVm(nrn);
		nrn.GiSyn = this.Act.GiFromSyn(ni, nrn, nrn.GiSyn, randctr);
	}

// reads pool Gi values
	void GInteg(int ni, inout Neuron nrn, inout Time ctime, inout uint2 randctr) {
		this.GFromSpikeRaw(ni, nrn, ctime);
		// note: can add extra values to GeRaw and GeSyn here
		this.GFromRawSyn(ni, nrn, ctime, randctr);
		this.GiInteg(ni, nrn, ctime);
	}

	void SpikeFromG(int ni, inout Neuron nrn, inout Time ctime) {
		float intdt = this.Act.Dt.IntDt;
		if ((1 == ctime.PlusPhase)) {
			intdt *= 3.0;
		}
		this.Act.VmFromG(nrn);
		this.Act.SpikeFromVm(nrn);
		this.Learn.CaFromSpike(nrn);
		if (ctime.Cycle >= this.Act.Dt.MaxCycStart) {
			nrn.SpkMaxCa += this.Learn.CaSpk.Dt.PDt * (nrn.CaSpkM - nrn.SpkMaxCa);
			if (nrn.SpkMaxCa > nrn.SpkMax) {
				nrn.SpkMax = nrn.SpkMaxCa;
			}
		}
		nrn.ActInt += intdt * (nrn.Act - nrn.ActInt); // using reg act here now
		if ((0 == ctime.PlusPhase)) {
			nrn.GeM += this.Act.Dt.IntDt * (nrn.Ge - nrn.GeM);
			nrn.GiM += this.Act.Dt.IntDt * (nrn.GiSyn - nrn.GiM);
		}
	}

	void CycleNeuron(int ni, inout Neuron nrn, inout Time ctime) {
		uint2 randctr = ctime.RandCtr.Uint2(); // use local var
		this.GInteg(ni, nrn, ctime, randctr);
		this.SpikeFromG(ni, nrn, ctime);
	}

	void CycleTimeInc(inout Time ctime) {
		ctime.CycleInc();
		ctime.RandCtr.Add(2); // main code uses fixed inc across all layers..
	}

};

// todo: why is this UpdateParams and not just Update()?

// UpdateParams updates all params given any changes that might have been made to individual values
// including those in the receiving projections of this layer

//////////////////////////////////////////////////////////////////////////////////////
//  Cycle



// from file: axon.hlsl

// note: on Mac can get away with 16 byte idx
// struct Index {
// 	uint X;
// 	uint Y;
// };

// note: binding is var, set
[[vk::binding(0, 0)]] RWStructuredBuffer<Layer> Layers;
[[vk::binding(0, 1)]] RWStructuredBuffer<Time> time;
[[vk::binding(0, 2)]] RWStructuredBuffer<Neuron> Neurons;
// [[vk::binding(0, 3)]] StructuredBuffer<Index> Indexes;
// note: uniform declaration for Indexes doesn't work

// note: the only way to get a local var to struct is via a function call param
void CycleNeuron(int ni, inout Neuron nrn, inout Time ctime) {
	Layers[nrn.LayIndex].CycleNeuron(ni, nrn, ctime);
	if(ni == 0) {
		Layers[nrn.LayIndex].CycleTimeInc(ctime);
		// updating time completely within this loop does NOT work
		// because the memory update is not shared!
	}
	// nrn.SpkSt1 = float(Indexes[ni].X); // debugging
	// nrn.SpkSt2 = float(nrn.LayIndex);
}

// important: this must be right before main, and 64 is typical default 
// number of procs per wave / warp (32 for NVIDIA & M1, 64 AMD)
[numthreads(64, 1, 1)]
void main(uint3 idx : SV_DispatchThreadID) {
	// use range checking if not guaranteeing sizes even multiple of numthreads
	// adds no perceptible time cost
	uint ns;
	uint st;
	Neurons.GetDimensions(ns, st);
	if(idx.x < ns) {
		CycleNeuron(idx.x, Neurons[idx.x], time[0]);
		/*
		int ni = idx.x;
		Layers[0].CycleNeuron(ni, Neurons[ni], time[0]);
		// if(ni == 0) {
		Layers[0].CycleTimeInc(time[0]);
		// }
		*/
	}
}

#endif // __AXON_HLSL__
