package outrightsmle

import (
	"strconv"
	"strings"
)

// SeasonPointsResult contains both expected points and the simulation used to calculate them
type SeasonPointsResult struct {
	ExpectedPoints map[string]float64
	SimPoints      *SimPoints
}

// calculateLeagueSeasonPointsWithSim calculates expected points using realistic fixture approach
// Returns both expected points and SimPoints for reuse in mark calculations
func calculateLeagueSeasonPointsWithSim(teamNames []string, params MLEParams, simParams *SimParams, 
	allEvents []MatchResult, league string, currentSeason string) *SeasonPointsResult {
	
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
	leagueTable := calcLeagueTable(teamNames, events)
	
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
	allEvents []MatchResult, league string, currentSeason string) map[string]float64 {
	result := calculateLeagueSeasonPointsWithSim(teamNames, params, simParams, allEvents, league, currentSeason)
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

// calculateMarkValues calculates mark values for markets using position probabilities from simulation
func calculateMarkValues(simPoints *SimPoints, markets []Market, league string) map[string]map[string]float64 {
	markValues := make(map[string]map[string]float64)
	
	// Filter markets for this league
	var leagueMarkets []Market
	for _, market := range markets {
		if market.League == league {
			leagueMarkets = append(leagueMarkets, market)
		}
	}
	
	if len(leagueMarkets) == 0 {
		return markValues
	}
	
	// Calculate mark value for each market
	for _, market := range leagueMarkets {
		teamMarks := make(map[string]float64)
		
		// Parse payoff structure (e.g., "1|4x0.25|19x0")
		payoffParts := parsePayoffStructure(market.Payoff)
		
		// Get position probabilities for teams eligible for this market (cached)
		marketPositionProbs := simPoints.positionProbabilities(market.Teams)
		
		// Calculate expected value ONLY for teams included in this market
		for _, teamName := range simPoints.TeamNames {
			// Check if this team is included in this market
			teamIncluded := false
			for _, includedTeam := range market.Teams {
				if includedTeam == teamName {
					teamIncluded = true
					break
				}
			}
			
			if teamIncluded {
				// Team is in the market - calculate expected value
				if teamProbs, exists := marketPositionProbs[teamName]; exists {
					expectedValue := 0.0
					
					// Calculate expected payout based on position probabilities
					for position, prob := range teamProbs {
						if position < len(payoffParts) {
							expectedValue += prob * payoffParts[position]
						}
					}
					
					teamMarks[teamName] = expectedValue
				}
			}
			// Teams excluded from market are not added to teamMarks (will be blank in display)
		}
		
		markValues[market.Name] = teamMarks
	}
	
	return markValues
}

// parsePayoffStructure parses payoff string like "1|4x0.25|19x0" into position-based payouts
// "1" means position 0 gets 1.0, "4x0.25" means positions 1,2,3,4 get 0.25, "19x0" means positions 5-23 get 0.0
func parsePayoffStructure(payoffStr string) []float64 {
	// Split by | to get position payoff groups
	parts := strings.Split(payoffStr, "|")
	
	// Calculate total positions needed
	totalPositions := 0
	for _, part := range parts {
		if strings.Contains(part, "x") {
			// Parse multiplier format: "4x0.25" means 4 positions
			multiplierParts := strings.Split(part, "x")
			if len(multiplierParts) == 2 {
				if count := parseInt(multiplierParts[0]); count > 0 {
					totalPositions += count
				}
			}
		} else {
			// Simple value like "1" means 1 position
			totalPositions++
		}
	}
	
	// Build payoff array
	payoffs := make([]float64, totalPositions)
	position := 0
	
	for _, part := range parts {
		if strings.Contains(part, "x") {
			// Parse multiplier format: "4x0.25" means 4 positions get 0.25
			multiplierParts := strings.Split(part, "x")
			if len(multiplierParts) == 2 {
				count := parseInt(multiplierParts[0])
				payout := parseFloat(multiplierParts[1])
				
				for i := 0; i < count && position < len(payoffs); i++ {
					payoffs[position] = payout
					position++
				}
			}
		} else {
			// Simple value like "1"
			if position < len(payoffs) {
				payoffs[position] = parseFloat(part)
				position++
			}
		}
	}
	
	return payoffs
}

// parseFloat safely parses a string to float64
func parseFloat(s string) float64 {
	if val, err := strconv.ParseFloat(s, 64); err == nil {
		return val
	}
	return 0.0
}

// parseInt safely parses a string to int
func parseInt(s string) int {
	if val, err := strconv.Atoi(s); err == nil {
		return val
	}
	return 0
}

// Additional metrics functions can be added here in the future:
// - calculateExpectedGoals()
// - calculateWinProbabilities()  
// - calculatePromotionRelegationProbabilities()
// - calculatePointsPerGame()
// etc.