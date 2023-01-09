# gosl

`gosl` implements Go as a shader language for GPU compute shaders: converts Go code to HLSL, and then uses the [glslc](https://github.com/google/shaderc) compiler (e.g., from a vulkan package) to compile into an `.spv` SPIR-V file that can be loaded into a vulkan compute shader.

Thus, `gosl` enables the same CPU-based Go code to also be run on the GPU.  The relevant subsets of Go code to use are specifically marked using `//gosl:` comment directives, and this code must only use basic expressions and concrete types that will compile correctly in a shader (see [Restrictions](#restrictions) below).  Method functions and pass-by-reference pointer arguments to `struct` types are supported and incur no additional compute cost due to inlining (see notes below for more detail).

See `examples/basic` for a working basic example, using the [vgpu](https://github.com/goki/vgpu) Vulkan-based GPU compute shader system.

Use these comment directives:

```
//gosl: start <filename>

< Go code to be translated >

//gosl: end <filename>
```

to bracket code to be processed.  The resulting converted code is copied into a `shaders` subdirectory created under the current directory where the `gosl` command is run, using the filenames specified in the comment directives.  Each such filename should correspond to a complete shader program, or a file that can be included into other shader programs.  Code is appended to the target file names in the order of the source .go files on the command line, so multiple .go files can be combined into one resulting HLSL file.

For the `main` HLSL function, global variables, to `#include` another `.hlsl` file, or other HLSL specific code, use the following comment directives:
```
//gosl: hlsl <filename>

// <HLSL shader code to be copied>

//gosl: end <filename>
```

where the HLSL shader code is commented out in the .go file -- it will be copied into the target filename and uncommented.  The HLSL code can be surrounded by `/*` `*/` comment blocks (each on a separate line) for multi-line code. 

Pass filenames, directory names, or Go package paths (e.g., `github.com/goki/mat32/fastexp.go` loads just that file from the given package) to `gosl` command for files to process -- files without any `//gosl:` comment directives will be skipped up front before any expensive processing, so it is not a problem to specify entire directories where only some files are relevant.

Usage:

	gosl [flags] [path ...]

The flags are:

    -exclude string
    	comma-separated list of names of functions to exclude from exporting to HLSL (default "Update,Defaults")
    -out string
    	output directory for shader code, relative to where gosl is invoked (default "shaders")
    -keep
    	keep temporary converted versions of the source files, for debugging

Any `struct` types encountered will be checked for 16-byte alignment of sub-types and overall sizes as an even multiple of 16 bytes (4 `float32` or `int32` values), which is the alignment used in HLSL and glsl shader languages, and the underlying GPU hardware presumably.  Look for error messages on the output from the gosl run.  This ensures that direct byte-wise copies of data between CPU and GPU will be successful.  The fact that `gosl` operates directly on the original CPU-side Go code uniquely enables it to perform these alignment checks, which are otherwise a major source of difficult-to-diagnose bugs.

You can safely ignore warnings of the form: `warning: Linking compute stage: Entry point not found` for any generated `.hlsl` files that serve only as includes to other files.
    
# Restrictions    

In general shader code should be simple mathematical expressions and data types, with minimal control logic via `if`, `for` statements, and only using the subset of Go that is consistent with C.  Here are specific restrictions:

## Types

* Can only use `float32`, `[u]int32`, and their 64 bit versions for basic types, and `struct` types composed of these same types -- no other Go types (i.e., `map`, slices, `string`, etc) are compatible.  There are strict alignment restrictions on 16 byte (e.g., 4 `float32`'s) intervals that are enforced via the `alignsl` sub-package.

* Use `slbool.Bool` instead of `bool` -- it defines a Go-friendly interface based on a `int32` basic type.  Using a `bool` in a `uniform` `struct` causes an obscure `glslc` compiler error: `shaderc: internal error: compilation succeeded but failed to optimize: OpFunctionCall Argument <id> '73[%73]'s type does not match Function`  

* Alignment and padding of `struct` fields is key -- todo: checker for compatibility.

* HLSL does not support enum types, but standard go `const` declarations will be converted.  Use an `int32` or `uint32` data type.  You cannot use `iota` -- value must be present in the Go source.  Also, for bitflags, define explicitly, not using `bitflags` package.

* HLSL does not do multi-pass compiling, so all dependent types must be specified *before* being used in other ones, and this also precludes referencing the *current* type within itself.  todo: can you just use a forward declaration?

## Syntax

* Cannot use multiple return values, or multiple assignment of variables in a single `=` expression.

* *Can* use multiple variable names with the same type (e.g., `min, max float32`) -- this will be properly converted to the more redundant C form with the type repeated.

## Random numbers

HLSL does not directly support random numbers.  Here's some discussion: [unity forum](https://forum.unity.com/threads/generate-random-float-between-0-and-1-in-shader.610810/)

# Implementation / Design Notes

HLSL is very C-like and provides a much better target for Go conversion than glsl.  See `examples/basic/shaders/basic_nouse.glsl` vs the .hlsl version there for the difference.  Only HLSL supports methods in a struct, and performance is the same as writing the expression directly -- it is suitably [inlined](https://learn.microsoft.com/en-us/windows/win32/direct3dhlsl/dx-graphics-hlsl-function-syntax).

While there aren't any pointers allowed in HLSL, the inlining of methods, along with the use of the `inout` [InputModifier](https://learn.microsoft.com/en-us/windows/win32/direct3dhlsl/dx-graphics-hlsl-function-parameters), effectively supports pass-by-reference.  The [stackoverflow](https://stackoverflow.com/questions/28527622/shaders-function-parameters-performance/28577878#28577878) on this is a bit unclear but the basic example demonstrates that it all goes through.


