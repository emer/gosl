# Makefile for glslc compiling of HLSL files for compute

all: axon.spv

%.spv : %.hlsl
	dxc -spirv -O3 -T cs_6_0 -E main -Fo $@ $<
	
