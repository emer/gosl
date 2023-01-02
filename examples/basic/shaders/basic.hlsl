
// DataStruct has the test data
struct DataStruct  {
	float Raw;
	float Integ;
	float Exp;
	float Pad2;
};

// ParamStruct has the test params
struct ParamStruct  {
	float Tau;
	float Dt;

// IntegFmRaw computes integrated value from current raw value
	void IntegFmRaw(inout DataStruct ds) {
		ds.Integ += this.Dt * (ds.Raw - ds.Integ);
		ds.Exp = exp(-ds.Integ);
	}

};


// note: double-commented lines required here -- binding is var, set
[[vk::binding(0, 0)]] uniform ParamStruct Params;
[[vk::binding(0, 1)]] RWStructuredBuffer<DataStruct> Data;
[numthreads(1, 1, 1)]
void main(uint3 idx : SV_DispatchThreadID) {
    Params.IntegFmRaw(Data[idx.x]);
}