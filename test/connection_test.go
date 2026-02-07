package test

import (
	"flag"
	"testing"
)

var (
	operatorMode = flag.Bool("operator", false, "Run tests in operator mode")
	staticMode   = flag.Bool("static", false, "Run tests in static mode")
)

func TestNatsConnectionStatic(t *testing.T) {
	if *staticMode {
		t.Log("Running tests in static mode")
		RunNatsConnectionTestProcedure(t, "nats-static-mode", "static", 4222)
	} else {
		t.Log("Do not run tests in static mode")
	}
}

func TestNatsConnectionOperator(t *testing.T) {
	if *operatorMode {
		t.Log("Running tests in operator mode")
		RunNatsConnectionTestProcedure(t, "nats-operator-mode", "operator", 4223)
	} else {
		t.Log("Do not run tests in operator mode")
	}
}
