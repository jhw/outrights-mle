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
			LambdaHome:    calculateExpectedGoals(params.AttackRatings[team], params.DefenseRatings[team], params.HomeAdvantage, true),
			LambdaAway:    calculateExpectedGoals(params.AttackRatings[team], params.DefenseRatings[team], params.HomeAdvantage, false),
		}
		teamRatings = append(teamRatings, rating)
	}

	result := &MLEResult{
		League:           request.League,
		Season:           request.Season,
		TeamRatings:      teamRatings,
		MLEParams:        *params,
		ProcessingTime:   time.Since(startTime),
		MatchesProcessed: len(request.HistoricalData),
	}

	return result, nil
}

// validateRequest checks if the MLE request is valid
func validateRequest(request MLERequest) error {
	if request.League == "" {
		return fmt.Errorf("league is required")
	}
	
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

// calculateExpectedGoals computes expected goals for a team - matches gist display calculation
func calculateExpectedGoals(attack, defense, homeAdv float64, isHome bool) float64 {
	// Match gist lines 447-448 and 467-468 exactly:
	// LambdaHome: math.Exp(attack + 0.3)
	// LambdaAway: math.Exp(attack) 
	if isHome {
		return math.Exp(attack + homeAdv)  // attack + 0.3
	}
	return math.Exp(attack) // just attack
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
	
	// Process each league
	leagues := []string{"ENG1", "ENG2", "ENG3", "ENG4"}
	for _, league := range leagues {
		leagueEvents, exists := eventsByLeague[league]
		if !exists {
			if options.Debug {
				fmt.Printf("⚠️  No events found for league %s\n", league)
			}
			continue
		}
		
		if options.Debug {
			fmt.Printf("\n🏈 Processing %s (%d events)...\n", league, len(leagueEvents))
		}
		
		// Sort events by date for consistent processing order
		sortedEvents := make([]MatchResult, len(leagueEvents))
		copy(sortedEvents, leagueEvents)
		sort.Slice(sortedEvents, func(i, j int) bool {
			return sortedEvents[i].Date < sortedEvents[j].Date
		})
		
		// Create MLE request for this league
		request := MLERequest{
			League:         league,
			Season:         latestSeason,
			HistoricalData: sortedEvents,
			PromotedTeams:  promotedTeams,
			LeagueGroups:   leagueGroups,
			Options:        options,
		}
		
		// Run MLE optimization
		leagueResult, err := OptimizeRatings(request)
		if err != nil {
			if options.Debug {
				fmt.Printf("❌ MLE optimization failed for %s: %v\n", league, err)
			}
			continue
		}
		
		if options.Debug {
			fmt.Printf("✅ %s optimization complete: %d iterations, converged=%v\n", 
				league, leagueResult.MLEParams.Iterations, leagueResult.MLEParams.Converged)
		}
		
		// Filter ratings based on league groups or latest season teams
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
			targetTeams = GetTeamsInSeason(leagueEvents, latestSeason)
			if options.Debug {
				fmt.Printf("📅 Using latest season teams: %d teams for %s\n", len(targetTeams), league)
			}
		}
		
		for _, rating := range leagueResult.TeamRatings {
			if _, isTargetTeam := targetTeams[rating.Team]; isTargetTeam {
				filteredRatings = append(filteredRatings, rating)
			}
		}
		
		// Sort by attack rating for consistent display
		sort.Slice(filteredRatings, func(i, j int) bool {
			return filteredRatings[i].AttackRating > filteredRatings[j].AttackRating
		})
		
		result.Leagues[league] = filteredRatings
	}
	
	result.ProcessingTime = time.Since(startTime)
	return result, nil
}

