package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	outrightsmle "github.com/jhw/go-outrights-mle/pkg/outrights-mle"
)

func main() {
	// Command line flags
	var (
		league      = flag.String("league", "ENG1", "League code (ENG1, ENG2, ENG3, ENG4)")
		season      = flag.String("season", "2023-24", "Season identifier")
		maxiter     = flag.Int("maxiter", 200, "Maximum MLE iterations")
		tolerance   = flag.Float64("tolerance", 1e-6, "Convergence tolerance")
		verbose     = flag.Bool("verbose", false, "Verbose output")
		dataFile    = flag.String("data", "", "Path to historical match data JSON file")
		fetchEvents = flag.Bool("fetch-events", false, "Fetch events data from football-data.co.uk and save to fixtures/events.json")
	)
	flag.Parse()

	fmt.Printf("🏈 Go Outrights MLE Demo\n")
	fmt.Printf("========================\n\n")

	// Handle fetch-events flag
	if *fetchEvents {
		fmt.Printf("🌐 Fetching events from football-data.co.uk...\n")
		
		events, err := FetchAllEvents()
		if err != nil {
			log.Fatalf("Failed to fetch events: %v", err)
		}

		// Save to fixtures/events.json
		eventsFile := "fixtures/events.json"
		if err := saveEventsToFile(events, eventsFile); err != nil {
			log.Fatalf("Failed to save events: %v", err)
		}

		fmt.Printf("\n✅ Successfully saved %d events to %s\n", len(events), eventsFile)
		return
	}

	// Set default file paths if not provided
	if *dataFile == "" {
		*dataFile = "fixtures/events.json" // Use real data if available
		
		// Check if events file exists, otherwise use sample data
		if _, err := os.Stat(*dataFile); os.IsNotExist(err) {
			*dataFile = fmt.Sprintf("fixtures/%s-matches.json", *league)
		}
	}

	// Load configuration and data
	fmt.Printf("Loading data for %s season %s...\n", *league, *season)

	var historicalData []outrightsmle.MatchResult
	var err error

	// Try to load real data first
	if *dataFile == "fixtures/events.json" {
		historicalData, err = loadEventsFromFile(*dataFile)
		if err != nil {
			fmt.Printf("⚠️  Could not load events file (%v), generating sample data instead\n", err)
			historicalData = generateSampleData(*league, *season)
		} else {
			// Filter for specific league if using real data
			if *league != "" {
				var filteredData []outrightsmle.MatchResult
				for _, match := range historicalData {
					if match.League == *league {
						filteredData = append(filteredData, match)
					}
				}
				historicalData = filteredData
			}
			fmt.Printf("✓ Loaded %d matches from %s\n", len(historicalData), *dataFile)
		}
	} else {
		// Generate sample data
		historicalData = generateSampleData(*league, *season)
		fmt.Printf("✓ Generated %d sample matches\n", len(historicalData))
	}

	// Set up MLE optimization options
	options := outrightsmle.MLEOptions{
		MaxIter:      *maxiter,
		Tolerance:    *tolerance,
		LearningRate: 0.1,
		TimeDecay:    0.78,
	}

	// Create MLE request
	request := outrightsmle.MLERequest{
		League:         *league,
		Season:         *season,
		HistoricalData: historicalData,
		Options:        options,
	}

	fmt.Printf("\nRunning MLE optimization...\n")
	fmt.Printf("- Maximum iterations: %d\n", *maxiter)
	fmt.Printf("- Convergence tolerance: %.2e\n", *tolerance)

	// Run the MLE optimization
	result, err := outrightsmle.OptimizeRatings(request)
	if err != nil {
		log.Fatalf("MLE optimization failed: %v", err)
	}

	fmt.Printf("\n✓ MLE optimization completed in %v\n", result.ProcessingTime)
	fmt.Printf("✓ Converged: %v (iterations: %d)\n", result.MLEParams.Converged, result.MLEParams.Iterations)
	fmt.Printf("✓ Log likelihood: %.2f\n", result.MLEParams.LogLikelihood)
	fmt.Printf("✓ Home advantage: %.3f\n", result.MLEParams.HomeAdvantage)

	// Sort teams by attack rating for better display
	sort.Slice(result.TeamRatings, func(i, j int) bool {
		return result.TeamRatings[i].AttackRating > result.TeamRatings[j].AttackRating
	})

	// Display results
	fmt.Printf("\n📊 Team Ratings\n")
	fmt.Printf("===============\n")
	fmt.Printf("%-20s %8s %8s %8s %8s\n", "Team", "Attack", "Defense", "λ_Home", "λ_Away")
	fmt.Printf("%-20s %8s %8s %8s %8s\n", "----", "------", "-------", "------", "------")

	for _, rating := range result.TeamRatings {
		fmt.Printf("%-20s %8.3f %8.3f %8.2f %8.2f\n",
			rating.Team,
			rating.AttackRating,
			rating.DefenseRating,
			math.Exp(rating.LambdaHome),
			math.Exp(rating.LambdaAway),
		)
	}

	// Display summary statistics
	fmt.Printf("\n📈 Summary Statistics\n")
	fmt.Printf("====================\n")
	
	var attackSum, defenseSum float64
	var attackMin, attackMax, defenseMin, defenseMax float64 = math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
	
	for _, rating := range result.TeamRatings {
		attackSum += rating.AttackRating
		defenseSum += rating.DefenseRating
		
		if rating.AttackRating < attackMin {
			attackMin = rating.AttackRating
		}
		if rating.AttackRating > attackMax {
			attackMax = rating.AttackRating
		}
		if rating.DefenseRating < defenseMin {
			defenseMin = rating.DefenseRating
		}
		if rating.DefenseRating > defenseMax {
			defenseMax = rating.DefenseRating
		}
	}
	
	numTeams := float64(len(result.TeamRatings))
	
	fmt.Printf("Attack ratings  - Mean: %6.3f, Range: [%6.3f, %6.3f]\n", 
		attackSum/numTeams, attackMin, attackMax)
	fmt.Printf("Defense ratings - Mean: %6.3f, Range: [%6.3f, %6.3f]\n", 
		defenseSum/numTeams, defenseMin, defenseMax)

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

	fmt.Printf("\n🎯 MLE optimization completed successfully!\n")
	fmt.Printf("   - Processed %d matches for %d teams\n", result.MatchesProcessed, len(result.TeamRatings))
	fmt.Printf("   - Ratings represent log-scale parameters for Poisson goal distributions\n")
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

// saveEventsToFile saves events to a JSON file
func saveEventsToFile(events []outrightsmle.MatchResult, filename string) error {
	// Create directories if they don't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	// Open file for writing
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", filename, err)
	}
	defer file.Close()

	// Encode JSON with indentation for readability
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(events); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}

	return nil
}

// loadEventsFromFile loads events from a JSON file
func loadEventsFromFile(filename string) ([]outrightsmle.MatchResult, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("opening file %s: %w", filename, err)
	}
	defer file.Close()

	var events []outrightsmle.MatchResult
	decoder := json.NewDecoder(file)

	if err := decoder.Decode(&events); err != nil {
		return nil, fmt.Errorf("decoding JSON: %w", err)
	}

	return events, nil
}