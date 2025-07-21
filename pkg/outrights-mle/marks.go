package outrightsmle

import (
	"strconv"
	"strings"
)

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