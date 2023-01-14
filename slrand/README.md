# slrand

This package contains HLSL header files and matching Go code for various random number generation (RNG) functions, which can be copied and `#include`d into an application's `shaders` directory.

We are using the [Philox2x32](https://github.com/DEShawResearch/random123) algorithm which is also available on CUDA on their [cuRNG](https://docs.nvidia.com/cuda/curand/host-api-overview.html) and in [Tensorflow](https://www.tensorflow.org/guide/random_numbers#general).  It is a counter based RNG ([CBRNG](https://en.wikipedia.org/wiki/Counter-based_random_number_generator_(CBRNG)) where the random number is a direct function of the input state, with no other internal state.  For a useful discussion of other alternatives, see [reddit cpp thread](https://www.reddit.com/r/cpp/comments/u3cnkk/old_rand_method_faster_than_new_alternatives/).  The code is based on the D.E. Shaw [github](https://github.com/DEShawResearch/random123/blob/main/include/Random123/philox.h) implementation.

The key advantage of this algorithm is its *stateless* nature, where the result is a deterministic but highly nonlinear function of its two inputs:
```
    uint2 res = Philox2x32(inout uint2 counter, uint key);
```
where the HLSL `uint2` type is 2 `uint32` 32-bit unsigned integers.  For GPU usage, the key is always set to the unique element being processed (e.g., the index of the data structure being updated), ensuring that different numbers are generated for each such element, and the counter should be configured as a shared global value that is incremented after every RNG call.  For example, if 4 RNG calls happen within a given set of GPU code, each thread starts with the same uniform starting value that will be incremented by 4 after the GPU call, and then it locally increments its local counter after each call.

The `Rand2x32` wrapper function around Philox2x32 will automatically increment the counter var passed to it, using the `CounterIncr()` method that manages the two 32 bit numbers as if they are a full 64 bit uint.

`gosl` will automatically translate the Go versions of the `slrand` package functions into their HLSL equivalents.

# slrand.hlsl

* `rand.Float32()` -> `rand32()`


# Implementational details

Unfortunately, vulkan `glslang` does not support 64 bit integers, even though the shader language model has somehow been updated to support them: https://github.com/KhronosGroup/glslang/issues/2965 --   https://github.com/microsoft/DirectXShaderCompiler/issues/2067.  This would also greatly speed up the impl: https://github.com/microsoft/DirectXShaderCompiler/issues/2821.

The result is that we have to use the slower version of the MulHiLo algorithm using only 32 bit uints.



