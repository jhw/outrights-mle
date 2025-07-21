package outrightsmle


// SeasonPointsResult contains both expected points and the simulation used to calculate them
type SeasonPointsResult struct {
	ExpectedPoints map[string]float64
	SimPoints      *SimPoints
}

// calculateLeagueSeasonPointsWithSim calculates expected points using realistic fixture approach
// Returns both expected points and SimPoints for reuse in mark calculations
func calculateLeagueSeasonPointsWithSim(teamNames []string, params MLEParams, simParams *SimParams, 
	allEvents []MatchResult, league string, currentSeason string, handicaps map[string]int) *SeasonPointsResult {
	
	// Use SimParams for simulation paths
	nPaths := simParams.SimulationPaths
	
	// Filter events for this league and current season
	var leagueEvents []MatchResult
	for _, event := range allEvents {
		if event.League == league && event.Season == currentSeason {
			leagueEvents = append(leagueEvents, event)
		}
	}
	
	// Convert to Event format for compatibility with go-outrights functions
	events := convertMatchResultsToEvents(leagueEvents, currentSeason)
	
	// Calculate current league table from existing matches
	leagueTable := calcLeagueTable(teamNames, events, handicaps)
	
	// Calculate remaining fixtures based on what's been played
	rounds := getRounds(league)
	remainingFixtures := calcRemainingFixtures(teamNames, events, rounds)
	
	// Initialize simulation points tracker with current league table
	simPoints := newSimPointsFromLeagueTable(leagueTable, nPaths, simParams.GoalDifferenceEffect)
	
	// Create a temporary solver for simulation with SimParams
	solver := &MLESolver{
		params:  &params,
		options: MLEOptions{SimParams: simParams},
	}
	
	// Simulate remaining fixtures and add to current points
	for _, fixtureName := range remainingFixtures {
		homeTeam, awayTeam := parseEventName(fixtureName)
		if homeTeam != "" && awayTeam != "" {
			simPoints.simulate(homeTeam, awayTeam, solver)
		}
	}
	
	// Calculate expected total season points (current + simulated remaining)
	expectedPoints := make(map[string]float64)
	for i, team := range leagueTable {
		total := 0.0
		for path := 0; path < nPaths; path++ {
			total += simPoints.Points[i][path]
		}
		expectedPoints[team.Name] = total / float64(nPaths)
	}
	
	return &SeasonPointsResult{
		ExpectedPoints: expectedPoints,
		SimPoints:      simPoints,
	}
}

// calculateLeagueSeasonPoints calculates expected points using realistic fixture approach
// Wrapper for backward compatibility
func calculateLeagueSeasonPoints(teamNames []string, params MLEParams, simParams *SimParams, 
	allEvents []MatchResult, league string, currentSeason string, handicaps map[string]int) map[string]float64 {
	result := calculateLeagueSeasonPointsWithSim(teamNames, params, simParams, allEvents, league, currentSeason, handicaps)
	return result.ExpectedPoints
}

// newSimPointsFromLeagueTable initializes SimPoints with current league table points (adapted from go-outrights)
func newSimPointsFromLeagueTable(leagueTable []Team, nPaths int, goalDifferenceEffect float64) *SimPoints {
	sp := &SimPoints{
		NPaths:        nPaths,
		TeamNames:     make([]string, len(leagueTable)),
		Points:        make([][]float64, len(leagueTable)),
		positionCache: make(map[string]map[string][]float64),
	}
	
	for i, team := range leagueTable {
		sp.TeamNames[i] = team.Name
		sp.Points[i] = make([]float64, nPaths)
		
		// Initialize with current points plus goal difference adjustments
		pointsWithAdjustments := float64(team.Points) + goalDifferenceEffect*float64(team.GoalDifference)
		
		for j := 0; j < nPaths; j++ {
			sp.Points[i][j] = pointsWithAdjustments
		}
	}
	
	return sp
}

// Additional team metrics functions can be added here in the future:
// - calculateExpectedGoals()
// - calculateWinProbabilities()  
// - calculatePromotionRelegationProbabilities()
// - calculatePointsPerGame()
// etc.