
// binding is var, set
[[vk::binding(0, 0)]] uniform sltype.Uint2 Counter;
[[vk::binding(0, 1)]] RWStructuredBuffer<Rnds> Data;

[numthreads(1, 1, 1)]
void main(uint3 idx : SV_DispatchThreadID) {
	Data[idx.x].RndGen(Counter, idx.x);
}


