package outrightsmle

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// OptimizeRatings runs the MLE-based team rating optimization
// This is the main entry point for the outrights-mle package
func OptimizeRatings(request MLERequest) (*MLEResult, error) {
	startTime := time.Now()

	// Validate input
	if err := validateRequest(request); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Apply defaults if not provided
	if request.Options == (MLEOptions{}) {
		request.Options = DefaultMLEOptions()
	}

	// Initialize MLE solver with historical data
	solver := NewMLESolver(request.HistoricalData, request.Options, request.PromotedTeams)

	// Run MLE optimization
	params, err := solver.Optimize()
	if err != nil {
		return nil, fmt.Errorf("MLE optimization failed: %w", err)
	}

	// Extract team ratings
	teamRatings := make([]TeamRating, 0, len(params.AttackRatings))
	for team := range params.AttackRatings {
		rating := TeamRating{
			Team:          team,
			AttackRating:  params.AttackRatings[team],
			DefenseRating: params.DefenseRatings[team],
			LambdaHome:    math.Exp(params.AttackRatings[team] + params.HomeAdvantage),  // attack + 0.3
			LambdaAway:    math.Exp(params.AttackRatings[team]),                        // just attack
		}
		teamRatings = append(teamRatings, rating)
	}

	result := &MLEResult{
		TeamRatings:      teamRatings,
		MLEParams:        *params,
		ProcessingTime:   time.Since(startTime),
		MatchesProcessed: len(request.HistoricalData),
	}

	return result, nil
}

// validateRequest checks if the MLE request is valid
func validateRequest(request MLERequest) error {
	if len(request.HistoricalData) == 0 {
		return fmt.Errorf("historical data is required")
	}

	// Validate that we have enough data
	if len(request.HistoricalData) < 100 {
		return fmt.Errorf("insufficient historical data: need at least 100 matches, got %d", len(request.HistoricalData))
	}

	// Check for required teams
	teams := extractTeams(request.HistoricalData)
	if len(teams) < 10 {
		return fmt.Errorf("insufficient teams: need at least 10 teams, got %d", len(teams))
	}

	return nil
}

// extractTeams gets unique team names from match data
func extractTeams(matches []MatchResult) []string {
	teamSet := make(map[string]bool)
	for _, match := range matches {
		teamSet[match.HomeTeam] = true
		teamSet[match.AwayTeam] = true
	}

	teams := make([]string, 0, len(teamSet))
	for team := range teamSet {
		teams = append(teams, team)
	}

	return teams
}


// MultiLeagueResult holds results for multiple leagues
type MultiLeagueResult struct {
	Leagues       map[string][]TeamRating `json:"leagues"`        // league -> team ratings
	LatestSeason  string                  `json:"latest_season"`  
	TotalMatches  int                     `json:"total_matches"`
	ProcessingTime time.Duration          `json:"processing_time"`
}

// ProcessMultipleLeagues processes events for multiple leagues and returns organized results
// This is the main high-level API for processing multi-league event data
func ProcessMultipleLeagues(events []MatchResult, options MLEOptions) (*MultiLeagueResult, error) {
	startTime := time.Now()
	
	if len(events) == 0 {
		return nil, fmt.Errorf("no events data provided")
	}
	
	// Initialize event processor
	processor := NewEventProcessor(events, options.Debug)
	
	// Load league groups (team configurations)
	if err := processor.LoadLeagueGroups(); err != nil {
		if options.Debug {
			fmt.Printf("⚠️  Could not load league groups: %v (will use latest season teams)\n", err)
		}
	}
	
	// Process events using the events module
	latestSeason := processor.FindLatestSeason()
	eventsByLeague := processor.GroupEventsByLeague()
	promotedTeams := processor.DetectPromotedTeams()
	leagueGroups := processor.GetLeagueGroups()
	
	result := &MultiLeagueResult{
		Leagues:        make(map[string][]TeamRating),
		LatestSeason:   latestSeason,
		TotalMatches:   len(events),
		ProcessingTime: time.Since(startTime),
	}
	
	// Sort all events by date for consistent processing order
	sort.Slice(events, func(i, j int) bool {
		return events[i].Date < events[j].Date
	})
	
	if options.Debug {
		fmt.Printf("\n🏈 Running single MLE optimization across ALL leagues (%d total events)...\n", len(events))
	}
	
	// Create single MLE request for ALL events across ALL leagues  
	request := MLERequest{
		HistoricalData: events,
		PromotedTeams:  promotedTeams,
		LeagueGroups:   leagueGroups,
		Options:        options,
	}
	
	// Run single MLE optimization across all leagues
	mlResult, err := OptimizeRatings(request)
	if err != nil {
		return nil, fmt.Errorf("MLE optimization failed: %w", err)
	}
	
	if options.Debug {
		fmt.Printf("✅ Single MLE optimization complete: %d iterations, converged=%v\n", 
			mlResult.MLEParams.Iterations, mlResult.MLEParams.Converged)
	}
	
	// Now filter and organize results by league
	leagues := []string{"ENG1", "ENG2", "ENG3", "ENG4"}
	for _, league := range leagues {
		if options.Debug {
			fmt.Printf("\n📊 Filtering results for %s...\n", league)
		}
		
		var filteredRatings []TeamRating
		var targetTeams map[string]bool
		
		// Use league groups if available, otherwise fall back to latest season teams
		if leagueGroups != nil && len(leagueGroups[league]) > 0 {
			targetTeams = make(map[string]bool)
			for _, team := range leagueGroups[league] {
				targetTeams[team] = true
			}
			if options.Debug {
				fmt.Printf("🎯 Using league groups: %d teams for %s\n", len(leagueGroups[league]), league)
			}
		} else {
			// Get teams from latest season for this league
			leagueEvents := eventsByLeague[league]
			if leagueEvents != nil {
				targetTeams = GetTeamsInSeason(leagueEvents, latestSeason)
				if options.Debug {
					fmt.Printf("📅 Using latest season teams: %d teams for %s\n", len(targetTeams), league)
				}
			}
		}
		
		// Filter ratings for this league and calculate expected season points
		var leagueTeams []string
		for _, rating := range mlResult.TeamRatings {
			if _, isTargetTeam := targetTeams[rating.Team]; isTargetTeam {
				filteredRatings = append(filteredRatings, rating)
				leagueTeams = append(leagueTeams, rating.Team)
			}
		}
		
		// Calculate expected season points for teams in this league
		expectedSeasonPoints := calculateLeagueSeasonPoints(leagueTeams, mlResult.MLEParams)
		
		// Update ratings with expected season points
		for i := range filteredRatings {
			if points, exists := expectedSeasonPoints[filteredRatings[i].Team]; exists {
				filteredRatings[i].ExpectedSeasonPoints = points
			}
		}
		
		// Sort by expected season points (descending) for league table order
		sort.Slice(filteredRatings, func(i, j int) bool {
			return filteredRatings[i].ExpectedSeasonPoints > filteredRatings[j].ExpectedSeasonPoints
		})
		
		result.Leagues[league] = filteredRatings
	}
	
	result.ProcessingTime = time.Since(startTime)
	return result, nil
}

// calculateLeagueSeasonPoints calculates expected points for a full season within one league
// Each team plays every other team both home and away
func calculateLeagueSeasonPoints(teams []string, params MLEParams) map[string]float64 {
	expectedPoints := make(map[string]float64)
	
	// Initialize all teams to 0 points
	for _, team := range teams {
		expectedPoints[team] = 0.0
	}
	
	// Create a temporary solver to use calculateExpectedMatchPoints
	solver := &MLESolver{
		params: &params,
	}
	
	// Simulate full season: each team plays every other team home and away
	for i, homeTeam := range teams {
		for j, awayTeam := range teams {
			if i != j { // Team doesn't play itself
				homePoints, awayPoints := solver.calculateExpectedMatchPoints(homeTeam, awayTeam)
				expectedPoints[homeTeam] += homePoints
				expectedPoints[awayTeam] += awayPoints
			}
		}
	}
	
	return expectedPoints
}

