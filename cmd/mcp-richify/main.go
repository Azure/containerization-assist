package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("  richify boundaries <output.json>  - Detect boundaries")
		fmt.Println("  richify convert <boundaries.json> - Convert errors")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "boundaries":
		if len(os.Args) != 3 {
			fmt.Println("Usage: richify boundaries <output.json>")
			os.Exit(1)
		}
		outputFile := os.Args[2]
		if err := runBoundaries(outputFile); err != nil {
			fmt.Printf("Error during boundary detection: %v\n", err)
			os.Exit(1)
		}
	case "convert":
		if len(os.Args) != 3 {
			fmt.Println("Usage: richify convert <boundaries.json>")
			os.Exit(1)
		}
		boundariesFile := os.Args[2]
		if err := runConvert(boundariesFile); err != nil {
			fmt.Printf("Error during conversion: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}
