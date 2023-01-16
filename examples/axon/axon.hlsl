
// note: binding is var, set
[[vk::binding(0, 0)]] uniform Layer Lay;
[[vk::binding(0, 1)]] RWStructuredBuffer<Time> time;
[[vk::binding(0, 2)]] RWStructuredBuffer<Neuron> Neurons;
[numthreads(1, 1, 1)]
void main(uint3 idx : SV_DispatchThreadID) {
	// for(int i = 0; i < 200; i++) { // 2x faster to do internally
	Lay.CycleNeuron(idx.x, Neurons[idx.x], time[0], time[0].RandCtr.Uint2());
	if(idx.x == 0) {
		Lay.CycleTimeInc(time[0]);
	}
	// }
}

