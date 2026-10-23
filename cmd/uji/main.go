// Command uji builds, stores and installs Android and iOS apps.
package main

import (
	"os"

	"github.com/ujizin/ujibox/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
