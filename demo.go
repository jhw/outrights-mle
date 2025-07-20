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

		// Run model and get team ratings by league
		teamRatingsByLeague, err := runMLEModel(events, *debug, *maxiter, *tolerance)
		if err != nil {
			log.Fatalf("MLE model failed: %v", err)
		}

		// Display results for latest season
		displayTeamRatingsByLeague(teamRatingsByLeague, *verbose)
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

// TeamRatingResult holds team ratings with league information
type TeamRatingResult struct {
	League     string
	Season     string
	TeamRating outrightsmle.TeamRating
}

// TeamConfig represents a team configuration from core-data
type TeamConfig struct {
	Name     string   `json:"name"`
	AltNames []string `json:"altNames,omitempty"`
}

// loadLeagueGroups loads team configurations from core-data/teams files
func loadLeagueGroups() (map[string][]string, error) {
	leagues := []string{"ENG1", "ENG2", "ENG3", "ENG4"}
	leagueGroups := make(map[string][]string)
	
	for _, league := range leagues {
		filename := fmt.Sprintf("core-data/%s-teams.json", league)
		
		// Check if file exists
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			// File doesn't exist, skip this league
			continue
		}
		
		file, err := os.Open(filename)
		if err != nil {
			return nil, fmt.Errorf("opening teams file %s: %w", filename, err)
		}
		defer file.Close()
		
		var teams []TeamConfig
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&teams); err != nil {
			return nil, fmt.Errorf("decoding teams JSON from %s: %w", filename, err)
		}
		
		// Extract team names
		var teamNames []string
		for _, team := range teams {
			teamNames = append(teamNames, team.Name)
		}
		
		leagueGroups[league] = teamNames
	}
	
	return leagueGroups, nil
}

// runMLEModel processes all events and returns team ratings grouped by league for latest season
func runMLEModel(events []outrightsmle.MatchResult, debug bool, maxiter int, tolerance float64) (map[string][]TeamRatingResult, error) {
	if len(events) == 0 {
		return nil, fmt.Errorf("no events data provided")
	}

	// Load league groups from core-data/teams files  
	leagueGroups, err := loadLeagueGroups()
	if err != nil {
		if debug {
			fmt.Printf("⚠️  Could not load league groups: %v (will use latest season teams)\n", err)
		}
		leagueGroups = nil // Will fallback to latest season teams
	} else if debug {
		fmt.Printf("📂 Loaded league groups: ")
		for league, teams := range leagueGroups {
			fmt.Printf("%s(%d teams) ", league, len(teams))
		}
		fmt.Printf("\n")
	}

	// Find latest season dynamically
	latestSeason := findLatestSeason(events)
	if debug {
		fmt.Printf("🔍 Latest season detected: %s\n", latestSeason)
	}

	// Group events by league
	eventsByLeague := groupEventsByLeague(events)
	if debug {
		fmt.Printf("🔍 Found events for leagues: %v\n", getMapKeys(eventsByLeague))
	}

	// Detect promoted/relegated teams across all data
	promotedTeams := detectPromotedTeams(events, debug)
	if debug {
		fmt.Printf("📊 Found %d teams with historical league changes\n", len(promotedTeams))
	}

	teamRatingsByLeague := make(map[string][]TeamRatingResult)

	// Process each league
	leagues := []string{"ENG1", "ENG2", "ENG3", "ENG4"}
	for _, league := range leagues {
		leagueEvents, exists := eventsByLeague[league]
		if !exists {
			if debug {
				fmt.Printf("⚠️  No events found for league %s\n", league)
			}
			continue
		}

		if debug {
			fmt.Printf("\n🏈 Processing %s (%d events)...\n", league, len(leagueEvents))
		}

		// Set up MLE options with debug flag
		options := outrightsmle.MLEOptions{
			MaxIter:      maxiter,
			Tolerance:    tolerance,
			LearningRate: 0.1,
			TimeDecay:    0.78,
			Debug:        debug,
		}

		// Create MLE request
		request := outrightsmle.MLERequest{
			League:         league,
			Season:         latestSeason,
			HistoricalData: leagueEvents,
			PromotedTeams:  promotedTeams, // Pass promoted teams for enhanced learning
			LeagueGroups:   leagueGroups,  // Pass league groups for team filtering
			Options:        options,
		}

		// Run MLE optimization
		result, err := outrightsmle.OptimizeRatings(request)
		if err != nil {
			if debug {
				fmt.Printf("❌ MLE optimization failed for %s: %v\n", league, err)
			}
			continue
		}

		if debug {
			fmt.Printf("✅ %s optimization complete: %d iterations, converged=%v\n", 
				league, result.MLEParams.Iterations, result.MLEParams.Converged)
		}

		// Filter ratings based on league groups or latest season teams
		var filteredRatings []TeamRatingResult
		var targetTeams map[string]bool
		
		// Use league groups if available, otherwise fall back to latest season teams
		if leagueGroups != nil && len(leagueGroups[league]) > 0 {
			targetTeams = make(map[string]bool)
			for _, team := range leagueGroups[league] {
				targetTeams[team] = true
			}
			if debug {
				fmt.Printf("🎯 Using league groups: %d teams for %s\n", len(leagueGroups[league]), league)
			}
		} else {
			targetTeams = getTeamsInSeason(leagueEvents, latestSeason)
			if debug {
				fmt.Printf("📅 Using latest season teams: %d teams for %s\n", len(targetTeams), league)
			}
		}

		for _, rating := range result.TeamRatings {
			if _, isTargetTeam := targetTeams[rating.Team]; isTargetTeam {
				filteredRatings = append(filteredRatings, TeamRatingResult{
					League:     league,
					Season:     latestSeason,
					TeamRating: rating,
				})
			}
		}

		teamRatingsByLeague[league] = filteredRatings
	}

	return teamRatingsByLeague, nil
}

// findLatestSeason finds the most recent season in the dataset
func findLatestSeason(events []outrightsmle.MatchResult) string {
	latestSeason := ""
	for _, event := range events {
		if event.Season > latestSeason {
			latestSeason = event.Season
		}
	}
	return latestSeason
}

// groupEventsByLeague separates events by league
func groupEventsByLeague(events []outrightsmle.MatchResult) map[string][]outrightsmle.MatchResult {
	grouped := make(map[string][]outrightsmle.MatchResult)
	for _, event := range events {
		grouped[event.League] = append(grouped[event.League], event)
	}
	return grouped
}

// detectPromotedTeams identifies teams that have changed leagues historically
func detectPromotedTeams(events []outrightsmle.MatchResult, debug bool) map[string]bool {
	// Build team league history: team -> season -> league
	teamLeagueHistory := make(map[string]map[string]string)
	
	for _, event := range events {
		// Initialize if needed
		if teamLeagueHistory[event.HomeTeam] == nil {
			teamLeagueHistory[event.HomeTeam] = make(map[string]string)
		}
		if teamLeagueHistory[event.AwayTeam] == nil {
			teamLeagueHistory[event.AwayTeam] = make(map[string]string)
		}
		
		// Record league for this season
		teamLeagueHistory[event.HomeTeam][event.Season] = event.League
		teamLeagueHistory[event.AwayTeam][event.Season] = event.League
	}

	// Find all unique seasons and sort them
	seasonsSet := make(map[string]bool)
	for _, event := range events {
		seasonsSet[event.Season] = true
	}
	
	var seasons []string
	for season := range seasonsSet {
		seasons = append(seasons, season)
	}
	sort.Strings(seasons)

	promotedTeams := make(map[string]bool)
	
	if debug {
		fmt.Printf("🔄 Detecting teams with league changes across %d seasons...\n", len(seasons))
	}

	// Check for league changes between consecutive seasons
	for team, seasonHistory := range teamLeagueHistory {
		var changes []string
		hasChanged := false
		
		for i := 0; i < len(seasons)-1; i++ {
			currentLeague := seasonHistory[seasons[i]]
			nextLeague := seasonHistory[seasons[i+1]]
			
			// Both seasons must have data
			if currentLeague != "" && nextLeague != "" && currentLeague != nextLeague {
				hasChanged = true
				direction := "📈" // promotion (lower league number)
				if nextLeague > currentLeague {
					direction = "📉" // relegation (higher league number)
				}
				changes = append(changes, fmt.Sprintf("%s %s→%s", direction, seasons[i], seasons[i+1]))
			}
		}
		
		if hasChanged {
			promotedTeams[team] = true
			if debug {
				fmt.Printf("  🔄 %s: %s\n", team, strings.Join(changes, ", "))
			}
		}
	}
	
	return promotedTeams
}

// getTeamsInSeason returns teams that played in a specific season for a league
func getTeamsInSeason(events []outrightsmle.MatchResult, season string) map[string]bool {
	teams := make(map[string]bool)
	for _, event := range events {
		if event.Season == season {
			teams[event.HomeTeam] = true
			teams[event.AwayTeam] = true
		}
	}
	return teams
}

// displayTeamRatingsByLeague prints team ratings grouped by league
func displayTeamRatingsByLeague(teamRatingsByLeague map[string][]TeamRatingResult, verbose bool) {
	leagues := []string{"ENG1", "ENG2", "ENG3", "ENG4"}
	
	for _, league := range leagues {
		ratings, exists := teamRatingsByLeague[league]
		if !exists || len(ratings) == 0 {
			continue
		}

		// Sort teams by attack rating (descending)
		sort.Slice(ratings, func(i, j int) bool {
			return ratings[i].TeamRating.AttackRating > ratings[j].TeamRating.AttackRating
		})

		fmt.Printf("\n🏆 %s (%d teams):\n", league, len(ratings))
		fmt.Printf("%-20s %8s %8s %8s %8s\n", "Team", "Attack", "Defense", "λ_Home", "λ_Away")
		fmt.Printf("%-20s %8s %8s %8s %8s\n", "----", "------", "-------", "------", "------")

		for _, rating := range ratings {
			fmt.Printf("%-20s %8.3f %8.3f %8.2f %8.2f\n",
				rating.TeamRating.Team,
				rating.TeamRating.AttackRating,
				rating.TeamRating.DefenseRating,
				math.Exp(rating.TeamRating.LambdaHome),
				math.Exp(rating.TeamRating.LambdaAway),
			)
		}
	}
}

// getMapKeys returns the keys of a string map
func getMapKeys(m map[string][]outrightsmle.MatchResult) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}