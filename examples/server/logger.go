package main

import "log"

type DemoLogger struct {
}

func (logger *DemoLogger) ERR(message string) {
	log.Printf("[ERR] %s\n", message)
}

func (logger *DemoLogger) INFO(message string) {
	log.Printf("[INFO] %s\n", message)
}
