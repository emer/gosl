# Makefile for glslc compiling of HLSL files for compute

all: axon.spv

%.spv : %.hlsl
	glslc -fshader-stage=compute -o $@ $<
	
