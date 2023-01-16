# Makefile for glslc compiling of HLSL files for compute

all: rand.spv

%.spv : %.hlsl
	glslc -fshader-stage=compute -o $@ $<
	
