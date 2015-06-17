package main

import (
	"log"
	"os"

	"github.com/deis/deis/builder"
)

func main() {
	os.Exit(builder.Run("boot"))
}
