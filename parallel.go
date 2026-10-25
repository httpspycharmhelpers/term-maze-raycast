package main

import "sync"

// parallelFor 把 [0,n) 按 CPU 核数分块并行执行；块间必须互不写同一内存。
// 小块直接串行，避免 goroutine 开销超过计算本身。
func parallelFor(n int, fn func(lo, hi int)) {
	workers := maxWorkers()
	if n < 64 || workers <= 1 {
		fn(0, n)
		return
	}
	if workers > n {
		workers = n
	}
	per := (n + workers - 1) / workers
	var wg sync.WaitGroup
	for k := 0; k < workers; k++ {
		lo := k * per
		if lo >= n {
			break
		}
		hi := lo + per
		if hi > n {
			hi = n
		}
		wg.Add(1)
		go func(lo, hi int) {
			defer wg.Done()
			fn(lo, hi)
		}(lo, hi)
	}
	wg.Wait()
}
