package sample

import (
	"log"
	"os"
)

func UsePanic() {
	panic("something went wrong") // want `use of built-in panic`
}

func UseOsExit() {
	os.Exit(1) // want `must not be called outside the main function of main package`
}

func UseLogFatal() {
	log.Fatal("fatal error") // want `must not be called outside the main function of main package`
}

func UseLogFatalf() {
	log.Fatalf("fatal: %v", "err") // want `must not be called outside the main function of main package`
}

func UseLogFatalln() {
	log.Fatalln("fatal error") // want `must not be called outside the main function of main package`
}
