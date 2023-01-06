package main

//gosl: start structs

type UintS1 struct {
	Field1 uint32
}

type UintS2 struct {
	Field1 uint32
	Field2 uint32
}

type UintS3 struct {
	Field1 uint32
	Field2 uint32
	Field3 uint32
}

type UintS4 struct {
	Field1 uint32
	Field2 uint32
	Field3 uint32
	Field4 uint32
}

type UintSC1 struct {
	Fla UintS1
	Flb UintS1
}

type UintSC2 struct {
	Fla UintS2
	Flb UintS2
}

type UintSC3 struct {
	Fla UintS3
	Flb UintS3
}

type UintSC4 struct {
	Fla UintS4
	Flb UintS4
}

//gosl: end structs

//gosl: hlsl structs
/*
[[vk::binding(0, 0)]] uniform UintS1 UiS1;
[[vk::binding(1, 0)]] uniform UintS2 UiS2;
[[vk::binding(2, 0)]] uniform UintS3 UiS3;
[[vk::binding(3, 0)]] uniform UintS4 UiS4;
[[vk::binding(4, 0)]] uniform UintSC1 UiSC1;
[[vk::binding(5, 0)]] uniform UintSC2 UiSC2;
[[vk::binding(6, 0)]] uniform UintSC3 UiSC3;
[[vk::binding(7, 0)]] uniform UintSC4 UiSC4;
[[vk::binding(0, 1)]] RWStructuredBuffer<UintS4> Res;

[numthreads(1, 1, 1)]
void main(uint3 idx : SV_DispatchThreadID) {
	Res[0].Field1 = UiS1.Field1;
	Res[1].Field1 = UiS2.Field1;
	Res[2].Field1 = UiS2.Field2;
	Res[3].Field1 = UiS3.Field1;
	Res[4].Field1 = UiS3.Field2;
	Res[5].Field1 = UiS3.Field3;
	Res[6].Field1 = UiS4.Field1;
	Res[7].Field1 = UiS4.Field2;
	Res[8].Field1 = UiS4.Field3;
	Res[9].Field1 = UiS4.Field4;
	Res[10].Field1 = UiSC1.Fla.Field1;
	Res[11].Field1 = UiSC1.Flb.Field1;
	Res[12].Field1 = UiSC2.Fla.Field1;
	Res[13].Field1 = UiSC2.Flb.Field1;
}
*/
//gosl: end structs
