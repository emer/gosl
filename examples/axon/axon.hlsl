
// note: binding is var, set
[[vk::binding(0, 0)]] uniform Layer Lays[];
[[vk::binding(0, 1)]] RWStructuredBuffer<Time> time;
[[vk::binding(0, 2)]] RWStructuredBuffer<Neuron> Neurons;

[numthreads(64, 1, 1)]

void main(uint3 idx : SV_DispatchThreadID) {
	// use range checking if not guaranteeing sizes even multiple of numthreads
	uint ns;
	uint st;
	Neurons.GetDimensions(ns, st);
	if(idx.x < ns) {
		Lays[Neurons[idx.x].LayIdx].CycleNeuron(idx.x, Neurons[idx.x], time[0]);
		if(idx.x == 0) {
			Lays[Neurons[idx.x].LayIdx].CycleTimeInc(time[0]);
		}
	}
}

