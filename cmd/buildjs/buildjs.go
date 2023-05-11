package main

import (
	"fmt"
	"os"

	"github.com/evanw/esbuild/pkg/api"
)

func main() {

	// check if js/node_modules dir exists
	// if not, prompt user to run npm install
	if _, err := os.Stat("./cmd/buildjs/js/node_modules"); os.IsNotExist(err) {
		fmt.Println("Please run `npm install` in cmd/buildjs/js first")
		os.Exit(1)
	}

	// use esbuild to bundle js
	result := api.Build(api.BuildOptions{
		EntryPoints: []string{"./cmd/buildjs/js/index.ts"},
		Outfile:     "public/js/index.js",
		Bundle:      true,
		Write:       true,
		Sourcemap:   api.SourceMapLinked,
	})

	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			os.Stderr.WriteString(err.Text)
		}
		os.Exit(1)
	}
}
