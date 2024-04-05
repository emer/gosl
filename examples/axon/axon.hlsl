
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

