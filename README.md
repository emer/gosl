# gosl

Go as a shader language: converts Go code to HLSL.  See `examples/basic` for a working basic example, using the [vgpu](https://github.com/goki/vgpu) Vulkan-based GPU compute shader system.

use:

```
//gosl: start <filename>

< Go code to be translated >

//gosl: end <filename>
```

to bracket code to be processed.  Everything is copied into `shaders` subdirectory created under the current directory where the command is run, using the filenames specified in the comment directives.  Each such filename should correspond to a complete shader program, or a file that can be included into other shader programs.

use:
```
//gosl: hlsl <filename>

// <HLSL shader code to be copied>

//gosl: end <filename>
```

for shader code that is commented out in the .go file, which will be copied into the filename
and uncommented.  This is used e.g., for the main function and global variables.

Pass filenames or directory names to `gosl` command for files to process.

Usage:

	gosl [flags] [path ...]

The flags are:

	-out string
	  	output directory for shader code, relative to where gosl is invoked (default "shaders")


# TODO

* float32 -> float
* order of type, name reversed
* no pointers 

* arguments for methods / functions are tricky -- no reference args: https://stackoverflow.com/questions/28527622/shaders-function-parameters-performance/28577878#28577878

so everything will have to be rewritten in the original source -- best to access the data directly from global variables??  a bit unclear what the compiler does..


