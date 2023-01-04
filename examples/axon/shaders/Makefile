# Makefile for glslc compiling of HLSL files for compute

all: axon.spv

%.spv : %.hlsl
	glslc -fshader-stage=compute -o $@ $<
	# glslc --target-env=vulkan1.2 --target-spv=spv1.3 -fshader-stage=compute -o $@ $<
	
