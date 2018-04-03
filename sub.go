// Copyright 2015 Apcera Inc. All rights reserved.

package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/go-nats"
	"github.com/nats-io/go-nats-streaming"
	"github.com/nats-io/go-nats/bench"
)

// Some sane defaults
const (
	DefaultNumMsgs            = 100000
	DefaultNumPubs            = 1
	DefaultNumSubs            = 0
	DefaultAsync              = false
	DefaultMessageSize        = 128
	DefaultIgnoreOld          = false
	DefaultMaxPubAcksInflight = 1000
	DefaultClientID           = "benchmark"
)

func usage() {
	log.Fatalf("Usage: stan-bench [-s server (%s)] [--tls] [-id CLIENT_ID] [-np NUM_PUBLISHERS] [-ns NUM_SUBSCRIBERS] [-n NUM_MSGS] [-ms MESSAGE_SIZE] [-csv csvfile] [-mpa MAX_NUMBER_OF_PUBLISHED_ACKS_INFLIGHT] [-io] [-a] <subject>\n", nats.DefaultURL)
}

var benchmark *bench.Benchmark

func main() {
	var clusterID string
	flag.StringVar(&clusterID, "c", "test-cluster", "The NATS Streaming cluster ID")
	flag.StringVar(&clusterID, "cluster", "test-cluster", "The NATS Streaming cluster ID")

	var urls = flag.String("s", "nats://nats_a:4222,nats://nats_b:4222,nats://nats_c:4222", "The NATS server URLs (separated by comma")
	var tls = flag.Bool("tls", false, "Use TLS secure sonnection")
	var numPubs = flag.Int("np", DefaultNumPubs, "Number of concurrent publishers")
	var numSubs = flag.Int("ns", DefaultNumSubs, "Number of concurrent subscribers")
	var numMsgs = flag.Int("n", DefaultNumMsgs, "Number of messages to publish")
	//var async = flag.Bool("a", DefaultAsync, "Async message publishing")
	var messageSize = flag.Int("ms", DefaultMessageSize, "Message size in bytes.")
	var ignoreOld = flag.Bool("io", DefaultIgnoreOld, "Subscribers ignore old messages")
	//var maxPubAcks = flag.Int("mpa", DefaultMaxPubAcksInflight, "Max number of published acks in flight")
	var clientID = flag.String("id", DefaultClientID, "Benchmark process base client ID")
	//var csvFile = flag.String("csv", "", "Save bench data to csv file")

	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		usage()
	}

	// Setup the option block
	opts := nats.GetDefaultOptions()
	opts.Servers = strings.Split(*urls, ",")
	for i, s := range opts.Servers {
		opts.Servers[i] = strings.Trim(s, " ")
	}

	opts.Secure = *tls

	benchmark = bench.NewBenchmark("NATS Streaming", *numSubs, *numPubs)

	var startwg sync.WaitGroup
	var donewg sync.WaitGroup

	donewg.Add(*numSubs)

	// Run Subscribers first
	startwg.Add(*numSubs)
	for i := 0; i < *numSubs; i++ {
		select {
		case  <-time.After(2 * time.Second):
			subID := fmt.Sprintf("%s-sub-%d", *clientID, i)
			go runSubscriber(&startwg, &donewg, opts, clusterID, *numMsgs, *messageSize, *ignoreOld, subID)

		}
	}
	startwg.Wait()
	//
	//// Now Publishers
	//startwg.Add(*numPubs)
	//pubCounts := bench.MsgsPerClient(*numMsgs, *numPubs)
	//for i := 0; i < *numPubs; i++ {
	//	pubID := fmt.Sprintf("%s-pub-%d", *clientID, i)
	//	go runPublisher(&startwg, &donewg, opts, clusterID, pubCounts[i], *messageSize, *async, pubID, *maxPubAcks)
	//}
	//
	//log.Printf("Starting benchmark [msgs=%d, msgsize=%d, pubs=%d, subs=%d]\n", *numMsgs, *messageSize, *numPubs, *numSubs)
	//
	startwg.Wait()
	donewg.Wait()
	//
	benchmark.Close()
	fmt.Print(benchmark.Report())
	//
	//if len(*csvFile) > 0 {
	//	csv := benchmark.CSV()
	//	ioutil.WriteFile(*csvFile, []byte(csv), 0644)
	//	fmt.Printf("Saved metric data in csv file %s\n", *csvFile)
	//}
}
//
//func runPublisher(startwg, donewg *sync.WaitGroup, opts nats.Options, clusterID string, numMsgs int, msgSize int, async bool, pubID string, maxPubAcksInflight int) {
//	nc, err := opts.Connect()
//	if err != nil {
//		log.Fatalf("Publisher %s can't connect: %v\n", pubID, err)
//	}
//	snc, err := stan.Connect(clusterID, pubID, stan.MaxPubAcksInflight(maxPubAcksInflight), stan.NatsConn(nc))
//	if err != nil {
//		log.Fatalf("Publisher %s can't connect: %v\n", pubID, err)
//	}
//
//	startwg.Done()
//
//	args := flag.Args()
//
//	subj := args[0]
//	var msg []byte
//	if msgSize > 0 {
//		msg = make([]byte, msgSize)
//	}
//	published := 0
//	start := time.Now()
//
//	if async {
//		ch := make(chan bool)
//		acb := func(lguid string, err error) {
//			published++
//			if published >= numMsgs {
//				ch <- true
//			}
//		}
//		for i := 0; i < numMsgs; i++ {
//			_, err := snc.PublishAsync(subj, msg, acb)
//			if err != nil {
//				log.Fatal(err)
//			}
//		}
//		<-ch
//	} else {
//		for i := 0; i < numMsgs; i++ {
//			fmt.Println("",time.Now())
//			err := snc.Publish(subj, msg)
//			if err != nil {
//				log.Fatal(err)
//			}
//			published++
//		}
//	}
//
//	benchmark.AddPubSample(bench.NewSample(numMsgs, msgSize, start, time.Now(), snc.NatsConn()))
//	snc.Close()
//	nc.Close()
//	donewg.Done()
//}

func runSubscriber(startwg, donewg *sync.WaitGroup, opts nats.Options, clusterID string, numMsgs int, msgSize int, ignoreOld bool, subID string) {
	nc, err := opts.Connect()
	if err != nil {
		log.Fatalf("Subscriber %s can't connect: %v\n", subID, err)
	}
	snc, err := stan.Connect(clusterID, subID, stan.NatsConn(nc))
	if err != nil {
		log.Fatalf("Subscriber %s can't connect: %v\n", subID, err)
	}

	args := flag.Args()
	subj := args[0]
	ch := make(chan time.Time, 2)

	received := 0
	mcb := func(msg *stan.Msg) {
		//go func() {
		//	fmt.Printf("ID:%s,MSG:%s,TIME:%s\n",subID,string(msg.Data) , time.Now())
		//}()
		received++
		if received == 1 {
			ch <- time.Now()
		}
		if received >= numMsgs {
			ch <- time.Now()
		}
	}

	//if ignoreOld {
		snc.Subscribe(subj, mcb)
	//} else {
	//	snc.Subscribe(subj, mcb, stan.DeliverAllAvailable())
	//}
	fmt.Printf("subscribe:%s:%s\n",subID,subj)

	start := <-ch
	end := <-ch
	fmt.Println("exec:" , numMsgs , end.Sub(start))

	benchmark.AddSubSample(bench.NewSample(numMsgs, msgSize, start, end, snc.NatsConn()))
	snc.Close()
	nc.Close()
	donewg.Done()
}
