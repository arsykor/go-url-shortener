package main

import (
	"log"
	"os"
)

// non-main function, os.Exit and log.Fatal here are forbidden
func helper() {
	os.Exit(1)       // want `must not be called outside the main function of main package`
	log.Fatal("bad") // want `must not be called outside the main function of main package`
}

func main() {
	os.Exit(0)         // ← no want: this is allowed
	log.Fatal("fatal") // ← no want: this is allowed
	panic("oh no")     // want `use of built-in panic`
}
