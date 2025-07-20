package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	outrightsmle "github.com/jhw/go-outrights-mle/pkg/outrights-mle"
)

func main() {
	// Command line flags
	var (
		league     = flag.String("league", "ENG1", "League code (ENG1, ENG2, ENG3, ENG4)")
		season     = flag.String("season", "2023-24", "Season identifier")
		npaths     = flag.Int("npaths", 5000, "Number of Monte Carlo simulation paths")
		maxiter    = flag.Int("maxiter", 200, "Maximum MLE iterations")
		tolerance  = flag.Float64("tolerance", 1e-6, "Convergence tolerance")
		verbose    = flag.Bool("verbose", false, "Verbose output")
		dataFile   = flag.String("data", "", "Path to historical match data JSON file")
		teamsFile  = flag.String("teams", "", "Path to teams JSON file")
		marketsFile = flag.String("markets", "", "Path to markets JSON file")
	)
	flag.Parse()

	fmt.Printf("🏈 Go Outrights MLE Demo\n")
	fmt.Printf("========================\n\n")

	// Set default file paths if not provided
	if *dataFile == "" {
		*dataFile = fmt.Sprintf("fixtures/%s-matches.json", *league)
	}
	if *teamsFile == "" {
		*teamsFile = fmt.Sprintf("core-data/%s.json", *league)
	}
	if *marketsFile == "" {
		*marketsFile = fmt.Sprintf("core-data/%s.json", *league)
	}

	// Load configuration and data
	fmt.Printf("Loading data for %s season %s...\n", *league, *season)

	// For demo purposes, create sample historical data
	historicalData := generateSampleData(*league, *season)
	fmt.Printf("✓ Generated %d sample matches\n", len(historicalData))

	// Create sample markets
	markets := generateSampleMarkets(*league)
	fmt.Printf("✓ Created %d betting markets\n", len(markets))

	// Set up simulation options
	options := outrightsmle.SimOptions{
		NPaths:       *npaths,
		MaxIter:      *maxiter,
		Tolerance:    *tolerance,
		LearningRate: 0.1,
		TimeDecay:    0.78,
	}

	// Create simulation request
	request := outrightsmle.SimulationRequest{
		League:         *league,
		Season:         *season,
		HistoricalData: historicalData,
		Markets:        markets,
		Options:        options,
	}

	fmt.Printf("\nRunning MLE optimization and Monte Carlo simulation...\n")
	fmt.Printf("- Maximum iterations: %d\n", *maxiter)
	fmt.Printf("- Simulation paths: %d\n", *npaths)
	fmt.Printf("- Convergence tolerance: %.2e\n", *tolerance)

	// Run the simulation
	result, err := outrightsmle.Simulate(request)
	if err != nil {
		log.Fatalf("Simulation failed: %v", err)
	}

	fmt.Printf("\n✓ Simulation completed in %v\n", result.ProcessingTime)
	fmt.Printf("✓ MLE converged: %v (iterations: %d)\n", result.MLEParams.Converged, result.MLEParams.Iterations)
	fmt.Printf("✓ Log likelihood: %.2f\n", result.MLEParams.LogLikelihood)

	// Display results
	fmt.Printf("\n📊 Team Ratings & Predictions\n")
	fmt.Printf("=============================\n")
	fmt.Printf("%-20s %8s %8s %8s %8s %8s\n", "Team", "Attack", "Defense", "Points", "Winner", "Releg")

	for _, rating := range result.TeamRatings {
		expectedPts := result.ExpectedPoints[rating.Team]
		posProbs := result.PositionProbs[rating.Team]
		
		winnerProb := 0.0
		relegProb := 0.0
		
		if len(posProbs) > 0 {
			winnerProb = posProbs[0] * 100
		}
		if len(posProbs) >= 3 {
			for i := len(posProbs) - 3; i < len(posProbs); i++ {
				relegProb += posProbs[i] * 100
			}
		}

		fmt.Printf("%-20s %8.3f %8.3f %8.1f %7.1f%% %6.1f%%\n",
			rating.Team,
			rating.AttackRating,
			rating.DefenseRating,
			expectedPts,
			winnerProb,
			relegProb,
		)
	}

	// Display market prices
	fmt.Printf("\n💰 Market Prices\n")
	fmt.Printf("================\n")
	for marketName, price := range result.MarketPrices {
		fmt.Printf("%-30s: %.1f%%\n", marketName, price*100)
	}

	if *verbose {
		// Output full results as JSON
		fmt.Printf("\n📋 Full Results (JSON)\n")
		fmt.Printf("======================\n")
		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			log.Printf("Error marshaling results: %v", err)
		} else {
			fmt.Println(string(jsonData))
		}
	}

	fmt.Printf("\n🎯 Demo completed successfully!\n")
}

// generateSampleData creates sample historical match data for demonstration
func generateSampleData(league, season string) []outrightsmle.MatchResult {
	// Sample Premier League teams
	teams := []string{
		"Arsenal", "Chelsea", "Liverpool", "Manchester City", "Manchester United",
		"Tottenham", "Newcastle", "Brighton", "West Ham", "Crystal Palace",
		"Fulham", "Brentford", "Wolves", "Everton", "Aston Villa",
		"Nottingham Forest", "Bournemouth", "Sheffield United", "Burnley", "Luton",
	}

	var matches []outrightsmle.MatchResult
	startDate := time.Date(2023, 8, 1, 0, 0, 0, 0, time.UTC)

	// Generate matches for each round (38 rounds in Premier League)
	for round := 0; round < 38; round++ {
		matchDate := startDate.AddDate(0, 0, round*7)
		
		// Generate 10 matches per round (20 teams = 10 matches)
		for i := 0; i < len(teams); i += 2 {
			if i+1 < len(teams) {
				// Simple score generation based on team strength
				homeGoals := generateGoals(teams[i], true)
				awayGoals := generateGoals(teams[i+1], false)
				
				matches = append(matches, outrightsmle.MatchResult{
					Date:      matchDate.Format("2006-01-02"),
					Season:    season,
					League:    league,
					HomeTeam:  teams[i],
					AwayTeam:  teams[i+1],
					HomeGoals: homeGoals,
					AwayGoals: awayGoals,
				})
			}
		}
		
		// Rotate teams for next round (round-robin)
		if len(teams) > 1 {
			teams = append([]string{teams[0]}, append(teams[2:], teams[1])...)
		}
	}

	return matches
}

// generateGoals creates realistic goal counts based on team name (simplified)
func generateGoals(team string, isHome bool) int {
	// Simple strength mapping based on team names
	strength := 1.0
	
	strongTeams := map[string]float64{
		"Manchester City": 2.5,
		"Arsenal":        2.3,
		"Liverpool":      2.2,
		"Chelsea":        1.8,
		"Manchester United": 1.7,
		"Tottenham":      1.6,
		"Newcastle":      1.5,
	}
	
	if s, exists := strongTeams[team]; exists {
		strength = s
	}
	
	// Home advantage
	if isHome {
		strength *= 1.3
	}
	
	// Simple Poisson-like generation (very simplified)
	if strength > 2.0 {
		return int(strength + 0.5)
	}
	return int(strength)
}

// generateSampleMarkets creates sample betting markets
func generateSampleMarkets(league string) []outrightsmle.Market {
	// Sample teams for markets
	teams := []string{
		"Arsenal", "Chelsea", "Liverpool", "Manchester City", "Manchester United", "Tottenham",
	}
	
	return []outrightsmle.Market{
		{
			Name:   "Premier League Winner",
			Payoff: "1.0|19x0",
			Teams:  teams,
			Type:   "winner",
		},
		{
			Name:   "Top 4 Finish",
			Payoff: "1.0|16x0", 
			Teams:  teams,
			Type:   "top4",
		},
		{
			Name:   "Relegation",
			Payoff: "1.0|17x0",
			Teams:  []string{"Burnley", "Sheffield United", "Luton"},
			Type:   "relegation",
		},
	}
}