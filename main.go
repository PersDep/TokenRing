package main

import (
	"flag"
	"time"
)

func main() {
	nodesAmountPtr := flag.Int("n", 0, "nodes amount")
	holdingTimePtr := flag.Int("t", 0, "holding time")
	flag.Parse()
	nodesAmount := *nodesAmountPtr

	// PortArr := make([]int, nodesAmount)
	// MaintArr := make([]int, nodesAmount)
	// for i := 0; i < nodesAmount; i++ {
	// 	PortArr[i] = 30000 + i
	// 	MaintArr[i] = 40000 + i
	// }

	quitChan := make(chan struct{})
	for i := 0; i < nodesAmount; i++ {
		go runTokenRing(i, nodesAmount, quitChan, time.Millisecond*time.Duration(*holdingTimePtr), i == 0)
	}
	for i := 0; i < nodesAmount; i++ {
		<-quitChan
	}
	time.Sleep(time.Millisecond * 100)
}
