package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

// working name rkrd

var (
	listeningPort = flag.String("port", "8080", "port for rkrd to listen on")
	outputFile    = flag.String("out", "", "output file location")
)

func main() {
	flag.Parse()

	if len(*listeningPort) == 0 {
		logrus.Error("must provide a valid listening port")
		os.Exit(1)
	}

	addr := fmt.Sprintf(":%s", *listeningPort)
	rkrd := NewRkrd(addr)
	if err := rkrd.Start(); err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
	logrus.Info("up and running")

	for {
		if err := rkrd.HandleConnection(); err != nil {
			logrus.Error(err)
		}
	}

	v := make(chan struct{})
	<-v
}
