# Cgo: Do not pass pointers as uintptr

When calling a Cgo function with a pointer, you *must* pass it as `unsafe.Pointer`, and cannot use `uintptr`. This is necessary because Go values on the stack can move. When Go needs to make the stack larger, it copies the stack to a new location. It then updates any pointers to values on the stack (which must themselves be on the stack). It cannot update `uintptr` values, since they are integers, not pointers. This causes C to access the wrong memory. This is documented in the [unsafe.Pointer rules: "Conversion of a uintptr back to Pointer is not valid in general."](https://golang.org/pkg/unsafe/#Pointer).

This repository contains a demonstration of this bug. It declares a C function `cfunc_uintptr(uintptr_t data, size_t length)`, and calls it from many Goroutines. The C function checks that memory referenced by the arguments contains a known string. This program fails about 50% of the time on my machine. Unfortunately, triggering this bug is a bit sensitive, and you may need to adjust the value `numInts` in `fillStackSpace` to get this bug to reliably trigger.

It can be helpful to set [`GODEBUG=efence=1`](https://golang.org/pkg/runtime/) when trying to reproduce this bug. It crashes the program much more reliably. I should also change `fillStackSpace` to be a recursive function, so the program can automatically probe many different stack sizes.


## Triggering the bug

1. Build it: `go build -gcflags -m -o cgouintptrbug .`. Verify that the data in `triggerBug` is allocated on the stack:
    ```
    ./cgouintptrbug.go:39:16: ([]byte)("hello world bytes") does not escape
    ```
2. Run it multiple times: `for i in $(seq 10); do ./cgouintptrbug || break; done`. It should fail:
    ```
    data=0xc0005a6787 ERROR: unexpected bytes after sleep
    ...
    FAILED failed goroutines=16
    ```


## Cgo arguments must be allocated on the heap

This rule means that arguments to Cgo functions must be allocated on the heap. The only safe workaround is to wrap the C API in an object that contains a single heap allocated value, then copy arguments into that value before calling into Cgo. For example, for compression APIs, you can allocate a buffer in Go, copy many small arguments into that single buffer, then call into Cgo once to compress the entire buffer. This also allows batching calls to Cgo, which are relatively expensive.

There is a [discussion on a Go issue](https://github.com/golang/go/issues/24450) about allowing Cgo functions to be marked as `go:noescape` in some cases. It was closed by the Go team basically saying this is not safe if C calls back into Go. This demonstration shows this is not safe even if C does not call into Go, in case the stack needs to grow. The "trick" mentioned in that issue is to pass arguments as uintptr. This does confuse Go into thinking the arguments do not escape. However, this demonstration proves this is not safe, and will eventually cause your program to access the wrong memory.


## This bug is very subtle and fragile

The bug happens when the call to the generated `_Cfunc_c_function` Cgo wrapper needs to grow the stack. In this case, the arguments to the function have already been evaluated, and are then incorrect when passed into C. If you have any other function calls that use more stack space before this dangerous call, then they will grow the stack and the bug will not happen. The stack also must have just the right size to make the stack grow, so it may not happen, depending on how the function is called.

For example, I wrote this while investigating [a bug in `github.com/DataDog/zstd`](https://github.com/DataDog/zstd/issues/90). In this case, the error was in [`zstd.Writer.Write` when calling `ZSTD_compressStream2_wrapper`](https://github.com/DataDog/zstd/blob/6791cb49a0c2828a206828968d52f07bc07075f8/zstd_stream.go#L173). However, nearly the exact same code was used in [`zstd.CompressLevel` when calling ZSTD_compress_wrapper](https://github.com/DataDog/zstd/blob/6791cb49a0c2828a206828968d52f07bc07075f8/zstd.go#L81). I spent a lot of time trying to reproduce the bug, until I figured out that  `CompressLevel` calls `mallocgc` to allocate the output buffer, which triggers stack growth instead. I was finally able to trigger this bug, but I needed to use a recursive function to use varying amounts of stack space. I also added a sleep in the C code, which gives more time for other threads to overwrite the the memory.

One useful trick for tracking this down was to use gdb to break on the `runtime.morestack_noctxt` function to figure out what things were growing the stack. Using `GODEBUG=efence=1` also makes it crash more reliably. In general, this was tricky to understand.


## unsafe.Pointer exception for passing uintptr to syscall.Syscall does not apply to Cgo

In the unsafe.Pointer rules, there is an exception for uintptr arguments for `syscall.Syscall`: "The compiler handles a Pointer converted to a uintptr in the argument list of a call to a *function implemented in assembly* by arranging that the referenced allocated object, if any, is retained and not moved until the call completes" (emphasis mine). My interpretation is that because Cgo functions are not implemented in assembly, this does not apply to Cgo functions.
