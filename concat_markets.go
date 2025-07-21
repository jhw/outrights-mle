package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	coreDataDir := "core-data"
	outputFile := "fixtures/markets.json"

	if err := os.MkdirAll("fixtures", 0755); err != nil {
		fmt.Printf("Error creating fixtures directory: %v\n", err)
		os.Exit(1)
	}

	pattern := filepath.Join(coreDataDir, "*-markets.json")
	marketFiles, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Printf("Error finding market files: %v\n", err)
		os.Exit(1)
	}

	sort.Strings(marketFiles)

	var allMarkets []map[string]interface{}

	for _, marketFile := range marketFiles {
		filename := filepath.Base(marketFile)
		league := strings.TrimSuffix(filename, "-markets.json")

		fmt.Printf("Processing %s (league: %s)\n", filename, league)

		data, err := os.ReadFile(marketFile)
		if err != nil {
			fmt.Printf("  Error reading %s: %v\n", marketFile, err)
			continue
		}

		var markets []map[string]interface{}
		if err := json.Unmarshal(data, &markets); err != nil {
			fmt.Printf("  Error parsing %s: %v\n", marketFile, err)
			continue
		}

		for i := range markets {
			markets[i]["league"] = league
			allMarkets = append(allMarkets, markets[i])
		}

		fmt.Printf("  Added %d markets for %s\n", len(markets), league)
	}

	fmt.Printf("\nSaving %d total markets to %s\n", len(allMarkets), outputFile)

	outputData, err := json.MarshalIndent(allMarkets, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outputFile, outputData, 0644); err != nil {
		fmt.Printf("Error writing output file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Markets concatenation complete!")

	leagueCounts := make(map[string]int)
	for _, market := range allMarkets {
		if league, ok := market["league"].(string); ok {
			leagueCounts[league]++
		}
	}

	fmt.Println("\nSummary by league:")
	var leagues []string
	for league := range leagueCounts {
		leagues = append(leagues, league)
	}
	sort.Strings(leagues)

	for _, league := range leagues {
		fmt.Printf("  %s: %d markets\n", league, leagueCounts[league])
	}
}