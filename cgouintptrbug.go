package main

// #include "cfunc.h"
import "C"
import (
	"fmt"
	"os"
	"unsafe"
)

func main() {
	const threads = 1000
	barrier := make(chan struct{})
	results := make(chan int, threads)

	for i := 0; i < threads; i++ {
		go triggerBug(barrier, results)
	}

	close(barrier)

	failedCount := 0
	for i := 0; i < threads; i++ {
		result := <-results
		if result != 0 {
			failedCount++
		}
	}

	status := "SUCCESS"
	if failedCount != 0 {
		status = "FAILED"
	}
	fmt.Printf("%s failed goroutines=%d\n", status, failedCount)
	os.Exit(failedCount)
}

func triggerBug(barrier <-chan struct{}, results chan<- int) {
	data := []byte("hello world bytes")

	// wait for all other goroutines: seems to make failure more likely (although not required)
	<-barrier

	before := uintptr(unsafe.Pointer(&data[0]))
	result := fillStackSpace(data)
	after := uintptr(unsafe.Pointer(&data[0]))
	if before != after {
		fmt.Printf("data moved before=0x%016x after=0x%016x\n", before, after)
	}

	results <- result
}

func fillStackSpace(data []byte) int {
	const fillValue = 1
	const numInts = 105
	var space [numInts]int64
	for i := range space {
		space[i] = fillValue
	}

	// if you change this call to callCgoSafe, it will work (but data will escape to heap)
	space[0] += int64(callCgoUnsafe(data))

	return int(space[0] - fillValue)
}

func callCgoUnsafe(data []byte) int {
	return int(C.cfunc_uintptr(C.uintptr_t(uintptr(unsafe.Pointer(&data[0]))), C.size_t(len(data))))
}

func callCgoSafe(data []byte) int {
	return int(C.cfunc_void(unsafe.Pointer(&data[0]), C.size_t(len(data))))
}
