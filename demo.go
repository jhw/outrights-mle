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
	"strings"
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
		debug       = flag.Bool("debug", false, "Enable debug output during MLE optimization")
		dataFile    = flag.String("data", "", "Path to historical match data JSON file")
		fetchEvents = flag.Bool("fetch-events", false, "Fetch events data from football-data.co.uk and save to fixtures/events.json")
		runModel    = flag.Bool("run-model", false, "Run MLE model on all leagues using events data")
	)
	flag.Parse()

	fmt.Printf("🏈 Go Outrights MLE Demo\n")
	fmt.Printf("========================\n\n")

	// Handle fetch-events flag
	if *fetchEvents {
		fmt.Printf("🌐 Fetch events feature temporarily disabled\n")
		return
	}

	// Handle run-model flag
	if *runModel {
		fmt.Printf("🧮 Running MLE model on all leagues...\n")
		
		// Load events data
		events, err := loadEventsFromFile("fixtures/events.json")
		if err != nil {
			log.Fatalf("Failed to load events data: %v", err)
		}

		// Run model and get teams by league
		teamsByLeague, err := runMLEModel(events, *debug, *maxiter, *tolerance)
		if err != nil {
			log.Fatalf("MLE model failed: %v", err)
		}

		// Display results for latest season
		displayTeamsByLeague(teamsByLeague, *verbose)
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

	// Set up MLE optimization options with custom SimParams
	simParams := outrightsmle.DefaultSimParams()
	simParams.MaxIterations = *maxiter
	simParams.Tolerance = *tolerance
	
	options := outrightsmle.MLEOptions{
		SimParams: simParams,
		Debug:     *debug,
	}

	// Create MLE request
	request := outrightsmle.MLERequest{
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

	// Sort teams by expected season points for league table order
	sort.Slice(result.Teams, func(i, j int) bool {
		return result.Teams[i].ExpectedSeasonPoints > result.Teams[j].ExpectedSeasonPoints
	})

	// Display results
	fmt.Printf("\n📊 Team Ratings\n")
	fmt.Printf("===============\n")
	fmt.Printf("%3s %-20s %5s %5s %5s %8s %8s %8s %8s %8s\n", 
		"Pos", "Team", "Pts", "GD", "Pld", "Attack", "Defense", "λ_Home", "λ_Away", "SeasonPts")
	fmt.Printf("%3s %-20s %5s %5s %5s %8s %8s %8s %8s %8s\n", 
		"---", "----", "---", "--", "---", "------", "-------", "------", "------", "---------")

	for i, team := range result.Teams {
		fmt.Printf("%3d %-20s %5d %5d %5d %8.3f %8.3f %8.2f %8.2f %8.1f\n",
			i+1, // Position index starting from 1
			team.Name,
			team.Points,
			team.GoalDifference,
			team.Played,
			team.AttackRating,
			team.DefenseRating,
			team.LambdaHome,
			team.LambdaAway,
			team.ExpectedSeasonPoints,
		)
	}

	// Display summary statistics
	fmt.Printf("\n📈 Summary Statistics\n")
	fmt.Printf("====================\n")
	
	var attackSum, defenseSum float64
	var attackMin, attackMax, defenseMin, defenseMax float64 = math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
	
	for _, team := range result.Teams {
		attackSum += team.AttackRating
		defenseSum += team.DefenseRating
		
		if team.AttackRating < attackMin {
			attackMin = team.AttackRating
		}
		if team.AttackRating > attackMax {
			attackMax = team.AttackRating
		}
		if team.DefenseRating < defenseMin {
			defenseMin = team.DefenseRating
		}
		if team.DefenseRating > defenseMax {
			defenseMax = team.DefenseRating
		}
	}
	
	numTeams := float64(len(result.Teams))
	
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
	fmt.Printf("   - Processed %d matches for %d teams\n", result.MatchesProcessed, len(result.Teams))
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

// TeamResult holds team data with league information
type TeamResult struct {
	League string
	Season string
	Team   outrightsmle.Team
}


// runMLEModel processes all events using the API and returns teams grouped by league
func runMLEModel(events []outrightsmle.MatchResult, debug bool, maxiter int, tolerance float64) (map[string][]TeamResult, error) {
	// Set up MLE options with custom SimParams
	simParams := outrightsmle.DefaultSimParams()
	simParams.MaxIterations = maxiter
	simParams.Tolerance = tolerance
	
	options := outrightsmle.MLEOptions{
		SimParams: simParams,
		Debug:     debug,
	}

	// Use the high-level API to run MLE optimization across all leagues
	result, err := outrightsmle.RunMLESolver(events, options)
	if err != nil {
		// Check if this is a wrapped validation error and provide helpful message
		if strings.Contains(err.Error(), "league groups validation failed") {
			fmt.Printf("❌ League groups configuration validation failed:\n")
			fmt.Printf("   Teams in core-data/*-teams.json files must exist in the event data.\n")
			fmt.Printf("   Error: %v\n", err)
			return nil, fmt.Errorf("invalid league groups configuration")
		}
		return nil, fmt.Errorf("MLE solver failed: %w", err)
	}

	// Convert API result to demo format for display compatibility
	teamsByLeague := make(map[string][]TeamResult)
	
	for league, teams := range result.Leagues {
		var convertedTeams []TeamResult
		for _, team := range teams {
			convertedTeams = append(convertedTeams, TeamResult{
				League: league,
				Season: result.LatestSeason,
				Team:   team,
			})
		}
		teamsByLeague[league] = convertedTeams
	}

	return teamsByLeague, nil
}


// displayTeamsByLeague prints teams grouped by league
func displayTeamsByLeague(teamsByLeague map[string][]TeamResult, verbose bool) {
	leagues := []string{"ENG1", "ENG2", "ENG3", "ENG4"}
	
	for _, league := range leagues {
		teams, exists := teamsByLeague[league]
		if !exists || len(teams) == 0 {
			continue
		}

		// Sort teams by expected season points (descending) for league table order
		sort.Slice(teams, func(i, j int) bool {
			return teams[i].Team.ExpectedSeasonPoints > teams[j].Team.ExpectedSeasonPoints
		})

		fmt.Printf("\n🏆 %s (%d teams):\n", league, len(teams))
		fmt.Printf("%3s %-20s %5s %5s %5s %8s %8s %8s %8s %8s\n", 
			"Pos", "Team", "Pts", "GD", "Pld", "Attack", "Defense", "λ_Home", "λ_Away", "SeasonPts")
		fmt.Printf("%3s %-20s %5s %5s %5s %8s %8s %8s %8s %8s\n", 
			"---", "----", "---", "--", "---", "------", "-------", "------", "------", "---------")

		for i, teamResult := range teams {
			team := teamResult.Team
			fmt.Printf("%3d %-20s %5d %5d %5d %8.3f %8.3f %8.2f %8.2f %8.1f\n",
				i+1, // Position index starting from 1
				team.Name,
				team.Points,
				team.GoalDifference,
				team.Played,
				team.AttackRating,
				team.DefenseRating,
				team.LambdaHome,
				team.LambdaAway,
				team.ExpectedSeasonPoints,
			)
		}
	}
}

// getMapKeys returns the keys of a string map
