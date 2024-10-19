// Program apcupsc queries apcupsd services and summarizes their status.
package main

import (
	"flag"
	"fmt"
	"log"
	"sync"
	"time"

	"zappem.net/pub/net/apcupsc"
)

var (
	target  = flag.String("target", "localhost", "server to query at --port (overridden by --network)")
	port    = flag.Int("port", apcupsc.APCUPSDPort, "port number to query")
	network = flag.String("network", "", "network to scan. Example: 192.168.1.0/24")
	timeout = flag.Duration("timeout", 5*time.Second, "timeout for connections")
)

func main() {
	flag.Parse()

	if *port != 0 {
		apcupsc.APCUPSDPort = *port
	}

	var targets = []string{fmt.Sprintf("%s:%d", *target, apcupsc.APCUPSDPort)}
	if *network != "" {
		targets = apcupsc.Scan(*network, *timeout)
		if len(targets) == 0 {
			log.Fatalf("no targets found in --network=%q", *network)
		}
	}

	var wg sync.WaitGroup
	for _, a := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := apcupsc.ParseTarget(a)
			if err != nil {
				log.Printf("%s: %v", a, err)
			} else {
				log.Printf("%s: %#v", a, v)
			}
		}()
	}
	wg.Wait()
}
