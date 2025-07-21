package outrightsmle

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// RunSimulation runs the MLE-based team rating optimization and simulation
// This is the main entry point for the outrights-mle package
func RunSimulation(request MLERequest) (*MLEResult, error) {
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
	solver := NewMLESolver(request.HistoricalData, request.Options, request.LeagueChangeTeams)

	// Run MLE optimization
	params, err := solver.Optimize()
	if err != nil {
		return nil, fmt.Errorf("MLE optimization failed: %w", err)
	}

	// Extract team ratings into Team objects (with empty league table fields)
	teams := make([]Team, 0, len(params.AttackRatings))
	for teamName := range params.AttackRatings {
		team := Team{
			Name:                 teamName,
			Points:               0,  // No league table data at this level
			GoalDifference:       0,  // No league table data at this level  
			Played:               0,  // No league table data at this level
			AttackRating:         params.AttackRatings[teamName],
			DefenseRating:        params.DefenseRatings[teamName],
			LambdaHome:           math.Exp(params.AttackRatings[teamName] + params.HomeAdvantage),  // attack + home advantage
			LambdaAway:           math.Exp(params.AttackRatings[teamName]),                         // just attack
			ExpectedSeasonPoints: 0,  // Will be calculated later at league level
		}
		teams = append(teams, team)
	}

	// Generate match odds per league for current season teams only
	matchOdds := generateFixturesPerLeague(teams, solver, request)

	result := &MLEResult{
		Teams:            teams,
		MatchOdds:        matchOdds,
		MLEParams:        *params,
		ProcessingTime:   time.Since(startTime),
		MatchesProcessed: len(request.HistoricalData),
	}

	return result, nil
}




// MultiLeagueResult holds results for multiple leagues
type MultiLeagueResult struct {
	Leagues       map[string][]Team                          `json:"leagues"`        // league -> teams with all data
	Markets       []Market                                   `json:"markets"`        // validated and initialized markets
	MarkValues    map[string]map[string]map[string]float64   `json:"mark_values"`    // league -> market -> team -> mark_value
	LatestSeason  string                                     `json:"latest_season"`  
	TotalMatches  int                                        `json:"total_matches"`
	ProcessingTime time.Duration                             `json:"processing_time"`
}

// RunMLESolver runs MLE optimization across all leagues and returns organized results
// This is the main high-level API for cross-league MLE optimization
func RunMLESolver(events []MatchResult, markets []Market, options MLEOptions, handicaps map[string]int) (*MultiLeagueResult, error) {
	startTime := time.Now()
	
	if len(events) == 0 {
		return nil, fmt.Errorf("no events data provided")
	}
	
	// Extract global entities for validation
	globalEntities := ExtractGlobalEntities(events)
	if options.Debug {
		fmt.Printf("🔍 Found %d teams, %d leagues, %d seasons in event data\n", 
			len(globalEntities.Teams), len(globalEntities.Leagues), len(globalEntities.Seasons))
	}
	
	// Initialize event processor
	processor := NewEventProcessor(events, options.Debug)
	
	// Load league groups (team configurations)
	if err := processor.LoadLeagueGroups(); err != nil {
		if options.Debug {
			fmt.Printf("⚠️  Could not load league groups: %v (will use latest season teams)\n", err)
		}
	}
	
	// Validate league groups if they were loaded
	leagueGroups := processor.GetLeagueGroups()
	if err := ValidateLeagueGroups(leagueGroups, globalEntities); err != nil {
		return nil, fmt.Errorf("league groups validation failed: %w", err)
	}
	
	// Process events using the events module
	latestSeason := processor.FindLatestSeason()
	eventsByLeague := processor.GroupEventsByLeague()
	leagueChangeTeams := processor.DetectLeagueChangeTeams()
	
	// If league groups are specified, set latest season to empty (not using season-based selection)
	effectiveLatestSeason := latestSeason
	if len(leagueGroups) > 0 {
		effectiveLatestSeason = ""
		if options.Debug {
			fmt.Printf("🎯 Using league groups - latest season set to empty (not using season-based team selection)\n")
		}
	}
	
	// Get current teams for market validation using our helper function
	currentTeams := GetCurrentTeams(leagueGroups, eventsByLeague, latestSeason)
	
	// Validate and initialize markets
	if len(markets) > 0 {
		err := validateAndInitializeMarkets(markets, currentTeams, eventsByLeague, effectiveLatestSeason)
		if err != nil {
			return nil, fmt.Errorf("market validation failed: %w", err)
		}
		if options.Debug {
			fmt.Printf("✅ Validated %d markets across leagues\n", len(markets))
		}
	}

	result := &MultiLeagueResult{
		Leagues:        make(map[string][]Team),
		Markets:        markets,
		MarkValues:     make(map[string]map[string]map[string]float64),
		LatestSeason:   effectiveLatestSeason,
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
		LeagueChangeTeams: leagueChangeTeams,
		LeagueGroups:   leagueGroups,
		Handicaps:      handicaps,
		Options:        options,
	}
	
	// Run single MLE optimization across all leagues
	mlResult, err := RunSimulation(request)
	if err != nil {
		return nil, fmt.Errorf("MLE optimization failed: %w", err)
	}
	
	if options.Debug {
		fmt.Printf("✅ Single MLE optimization complete: %d iterations, converged=%v\n", 
			mlResult.MLEParams.Iterations, mlResult.MLEParams.Converged)
	}
	
	// Now filter and organize results by league - use leagues found in events
	leagues := ExtractLeagues(events)
	for _, league := range leagues {
		if options.Debug {
			fmt.Printf("\n📊 Filtering results for %s...\n", league)
		}
		
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
		
		// Filter teams for this league and collect team names
		var leagueTeams []string
		teamDataMap := make(map[string]Team)
		for _, team := range mlResult.Teams {
			if _, isTargetTeam := targetTeams[team.Name]; isTargetTeam {
				leagueTeams = append(leagueTeams, team.Name)
				teamDataMap[team.Name] = team
			}
		}
		
		// Calculate expected season points for teams in this league (with simulation reuse)
		seasonResult := calculateLeagueSeasonPointsWithSim(leagueTeams, mlResult.MLEParams, options.SimParams, 
			events, league, effectiveLatestSeason, request.Handicaps)
		expectedSeasonPoints := seasonResult.ExpectedPoints
		
		// Get current season matches for this league to build proper league table
		var leagueEvents []MatchResult
		for _, event := range events {
			if event.League == league && event.Season == effectiveLatestSeason {
				leagueEvents = append(leagueEvents, event)
			}
		}
		
		// Convert to Event format and calculate league table
		currentSeasonEvents := convertMatchResultsToEvents(leagueEvents, effectiveLatestSeason)
		leagueTable := calcLeagueTable(leagueTeams, currentSeasonEvents, request.Handicaps)
		
		// Create unified Team objects with all data
		var teams []Team
		for _, tableTeam := range leagueTable {
			if teamData, exists := teamDataMap[tableTeam.Name]; exists {
				team := Team{
					Name:           tableTeam.Name,
					Points:         tableTeam.Points,
					GoalDifference: tableTeam.GoalDifference,
					Played:         tableTeam.Played,
					AttackRating:   teamData.AttackRating,
					DefenseRating:  teamData.DefenseRating,
					LambdaHome:     teamData.LambdaHome,
					LambdaAway:     teamData.LambdaAway,
				}
				
				// Add expected season points
				if points, exists := expectedSeasonPoints[team.Name]; exists {
					team.ExpectedSeasonPoints = points
				}
				
				teams = append(teams, team)
			}
		}
		
		// Sort by expected season points (descending) for league table order
		sort.Slice(teams, func(i, j int) bool {
			return teams[i].ExpectedSeasonPoints > teams[j].ExpectedSeasonPoints
		})
		
		result.Leagues[league] = teams
		
		// Calculate mark values using the same simulation (reuse for performance)
		if len(markets) > 0 && seasonResult.SimPoints != nil {
			leagueMarkValues := calculateMarkValues(seasonResult.SimPoints, markets, league)
			if len(leagueMarkValues) > 0 {
				result.MarkValues[league] = leagueMarkValues
				if options.Debug {
					fmt.Printf("📊 Calculated mark values for %d markets in %s\n", len(leagueMarkValues), league)
				}
				
			}
		}
	}
	
	result.ProcessingTime = time.Since(startTime)
	return result, nil
}


// convertMatchResultsToEvents converts MatchResult to Event format
func convertMatchResultsToEvents(matches []MatchResult, season string) []Event {
	var events []Event
	
	for _, match := range matches {
		// Only include matches from the specified season
		if season != "" && match.Season != season {
			continue
		}
		
		event := Event{
			Name: match.HomeTeam + " vs " + match.AwayTeam,
			Date: match.Date,
			Score: []int{match.HomeGoals, match.AwayGoals},
		}
		events = append(events, event)
	}
	
	return events
}

// getRounds determines number of rounds based on league (SCO=2, others=1)
func getRounds(league string) int {
	if strings.HasPrefix(league, "SCO") {
		return 2
	}
	return 1
}


// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// generateFixturesPerLeague generates match odds for teams within their respective leagues only
// Uses leagueGroups if available, otherwise falls back to latest season teams
func generateFixturesPerLeague(teams []Team, solver *MLESolver, request MLERequest) []MatchOdds {
	var matchOdds []MatchOdds
	
	// Create a map of team name to Team for efficient lookup
	teamMap := make(map[string]Team)
	for _, team := range teams {
		teamMap[team.Name] = team
	}
	
	// Determine current teams per league
	processor := NewEventProcessor(request.HistoricalData, false)
	eventsByLeague := processor.GroupEventsByLeague()
	latestSeason := processor.FindLatestSeason()
	currentTeams := GetCurrentTeams(request.LeagueGroups, eventsByLeague, latestSeason)
	
	// Generate fixtures for each league separately
	for league, leagueTeams := range currentTeams {
		// Filter teams that exist in our optimized ratings
		var validTeams []Team
		for _, teamName := range leagueTeams {
			if team, exists := teamMap[teamName]; exists {
				validTeams = append(validTeams, team)
			}
		}
		
		// Generate all combinations within this league
		for i, homeTeam := range validTeams {
			for j, awayTeam := range validTeams {
				if i != j { // Skip same team vs same team
					fixture := fmt.Sprintf("%s vs %s", homeTeam.Name, awayTeam.Name)
					probabilities := solver.CalculateMatchProbabilities(homeTeam.Name, awayTeam.Name)
					
					matchOdds = append(matchOdds, MatchOdds{
						Fixture:       fixture,
						League:        league,
						Probabilities: probabilities,
					})
				}
			}
		}
	}
	
	return matchOdds
}


