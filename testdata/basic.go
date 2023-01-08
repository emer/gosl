package test

import (
	"math"

	"github.com/goki/gosl/slbool"
	"github.com/goki/mat32"
)

//gosl: start basic

// FastExp is a quartic spline approximation to the Exp function, by N.N. Schraudolph
// It does not have any of the sanity checking of a standard method -- returns
// nonsense when arg is out of range.  Runs in 2.23ns vs. 6.3ns for 64bit which is faster
// than math32.Exp actually.
func FastExp(x float32) float32 {
	if x <= -88.76731 { // this doesn't add anything and -exp is main use-case anyway
		return 0
	}
	i := int32(12102203*x) + 127*(1<<23)
	m := i >> 7 & 0xFFFF // copy mantissa
	i += (((((((((((3537 * m) >> 16) + 13668) * m) >> 18) + 15817) * m) >> 14) - 80470) * m) >> 11)
	return math.Float32frombits(uint32(i))
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

// DataStruct has the test data
type DataStruct struct {
	Raw   float32 `desc:"raw value"`
	Integ float32 `desc:"integrated value"`
	Exp   float32 `desc:"exp of integ"`
	Pad2  float32 `desc:"must pad to multiple of 4 floats for arrays"`
}

// ParamStruct has the test params
type ParamStruct struct {
	Tau    float32     `desc:"rate constant in msec"`
	Dt     float32     `desc:"1/Tau"`
	Option slbool.Bool // note: standard bool doesn't work
	// pad    float32 // comment this out to trigger alignment warning
}

func (ps *ParamStruct) IntegFmRaw(ds *DataStruct, modArg *float32) {
	// note: the following are just to test basic control structures
	newVal := ps.Dt*(ds.Raw-ds.Integ) + *modArg
	if newVal < -10 || slbool.IsTrue(ps.Option) {
		newVal = -10
	}
	ds.Integ += newVal
	ds.Exp = mat32.Exp(-ds.Integ)
}

// AnotherMeth does more computation
func (ps *ParamStruct) AnotherMeth(ds *DataStruct) {
	var i int
	for i = 0; i < 10; i++ {
		ds.Integ *= 0.99
	}
	var flag NeuronFlags
	flag &^= NeuronHasExt // clear flag -- op doesn't exist in C
}

//gosl: end basic

// note: only core compute code needs to be in shader -- all init is done CPU-side

func (ps *ParamStruct) Defaults() {
	ps.Tau = 5
	ps.Update()
}

func (ps *ParamStruct) Update() {
	ps.Dt = 1.0 / ps.Tau
}

//gosl: hlsl basic
/*
[[vk::binding(0, 0)]] uniform ParamStruct Params;
[[vk::binding(0, 1)]] RWStructuredBuffer<DataStruct> Data;
[numthreads(1, 1, 1)]
void main(uint3 idx : SV_DispatchThreadID) {
    Params.IntegFmRaw(Data[idx.x], Data[idx.x].Pad2);
}
*/
//gosl: end basic
