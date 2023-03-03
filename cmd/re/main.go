package main

import (
	"flag"

	"re/pkg/re"
)

func main() {
	targetPath := ""
	flag.StringVar(&targetPath, "t", "", "target path")

	flag.Parse()

	if targetPath == "" {
		targetPath = "."
	}

	re.Run(targetPath)
}
