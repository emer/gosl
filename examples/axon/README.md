# axon

This example tests the [axon](https://github.com/emer/axon) `CycleNeuron` function that implements a full conductance-based biologically realistic and detailed model of how a cortical pyramidal neuron responds to excitatory and inhibitory inputs.  In addition to basic excitation, inhibition, and leak channels, there are a number of active gated channels such as NMDA, GABA-B, M-type mAHP, etc channels that are a function of membrane potential (`Vm`) and other factors such as calcium (`Ca`) and sodium (`Na`).

The equations are much more complex compared to typical GPU-based matrix algebra (e.g., a dot product), and the parameter data structures include many 10's of float32 values, providing a good test of Go -> HLSL parsing and alignment checking, so that the resulting `struct` values can be directly copied from CPU to GPU.

The actual computation is relatively simple: an array of `Neuron` structures is allocated, initialized, and copied from CPU to GPU.  Then the overall `CycleNeuron` method is called repeatedly, for 200 cycles, which is the typical number of iterations per functional trial in axon.

A comparison of the CPU and GPU results are printed, along with timing.

All of the neurons receive the same excitatory input, and thus have the same behavior.

# Building

There is a `//go:generate` comment directive in `main.go` that calls `gosl` on the relevant files, so you can do `go generate` followed by `go build` to run it.  There is also a `Makefile` with the same `gosl` command, so `make` can be used instead of go generate.

The generated files go into the `shaders/` subdirectory.

The generate step must be re-run if any of the computation-relevant code is changed (i.e., within the `//gsl:` start / end blocks) but e.g., changing the number of neurons in `main.go` does not require a re-generate.

# TODO:

The time update is not working on linux -- only on mac -- probably something about shared memory.

