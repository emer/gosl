# gosl

Go as a shader language: converts go code to HLSL

# TODO

* float32 -> float
* order of type, name reversed
* no pointers 

* arguments for methods / functions are tricky -- no reference args: https://stackoverflow.com/questions/28527622/shaders-function-parameters-performance/28577878#28577878

so everything will have to be rewritten in the original source -- best to access the data directly from global variables??  a bit unclear what the compiler does..


