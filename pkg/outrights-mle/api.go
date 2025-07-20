package outrightsmle

import (
	"fmt"
	"time"
)

// Simulate runs the MLE-based football prediction simulation
// This is the main entry point for the outrights-mle package
func Simulate(request SimulationRequest) (*SimulationResult, error) {
	startTime := time.Now()

	// Validate input
	if err := validateRequest(request); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Apply defaults if not provided
	if request.Options == (SimOptions{}) {
		request.Options = DefaultSimOptions()
	}

	// Initialize MLE solver with historical data
	solver := NewMLESolver(request.HistoricalData, request.Options)

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

	// Initialize simulator with the optimized parameters
	simulator := NewSimulator(params, request.Options)

	// Run Monte Carlo simulation
	positionProbs, expectedPoints := simulator.SimulateSeason(extractTeams(request.HistoricalData))

	// Calculate market prices
	marketPrices := make(map[string]float64)
	for _, market := range request.Markets {
		price := calculateMarketPrice(market, positionProbs)
		marketPrices[market.Name] = price
	}

	result := &SimulationResult{
		League:           request.League,
		Season:           request.Season,
		TeamRatings:      teamRatings,
		MLEParams:        *params,
		ExpectedPoints:   expectedPoints,
		PositionProbs:    positionProbs,
		MarketPrices:     marketPrices,
		ProcessingTime:   time.Since(startTime),
		MatchesProcessed: len(request.HistoricalData),
	}

	return result, nil
}

// validateRequest checks if the simulation request is valid
func validateRequest(request SimulationRequest) error {
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

// calculateExpectedGoals computes expected goals for a team
func calculateExpectedGoals(attack, defense, homeAdv float64, isHome bool) float64 {
	lambda := attack - defense
	if isHome {
		lambda += homeAdv
	}
	return lambda
}

// calculateMarketPrice computes the fair price for a betting market
func calculateMarketPrice(market Market, positionProbs map[string][]float64) float64 {
	// This is a placeholder - actual implementation would depend on market type
	// and payoff structure parsing
	switch market.Type {
	case "winner":
		return calculateWinnerPrice(market, positionProbs)
	case "top4":
		return calculateTop4Price(market, positionProbs)
	case "relegation":
		return calculateRelegationPrice(market, positionProbs)
	default:
		return 0.0
	}
}

// Placeholder market calculation functions
func calculateWinnerPrice(market Market, positionProbs map[string][]float64) float64 {
	totalProb := 0.0
	for _, team := range market.Teams {
		if probs, exists := positionProbs[team]; exists && len(probs) > 0 {
			totalProb += probs[0] // Probability of finishing 1st
		}
	}
	return totalProb
}

func calculateTop4Price(market Market, positionProbs map[string][]float64) float64 {
	totalProb := 0.0
	for _, team := range market.Teams {
		if probs, exists := positionProbs[team]; exists && len(probs) >= 4 {
			for i := 0; i < 4; i++ {
				totalProb += probs[i] // Sum probabilities of finishing 1st-4th
			}
		}
	}
	return totalProb
}

func calculateRelegationPrice(market Market, positionProbs map[string][]float64) float64 {
	totalProb := 0.0
	for _, team := range market.Teams {
		if probs, exists := positionProbs[team]; exists {
			numPositions := len(probs)
			if numPositions >= 3 {
				// Sum probabilities of finishing in bottom 3
				for i := numPositions - 3; i < numPositions; i++ {
					totalProb += probs[i]
				}
			}
		}
	}
	return totalProb
}