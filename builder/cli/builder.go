package main

import (
	"os"

	"github.com/deis/deis/builder"
)

func main() {
	os.Exit(builder.Run("boot"))
}
