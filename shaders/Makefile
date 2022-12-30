# Makefile for glslc compiling of HLSL files for compute

all: basic.spv

%.spv : %.hlsl
	glslc -fshader-stage=compute -o $@ $<

