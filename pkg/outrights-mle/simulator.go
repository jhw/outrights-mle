package outrightsmle

import (
	"math"
	"math/rand"
	"sort"
	"time"
)

// Simulator runs Monte Carlo simulations for season outcomes
type Simulator struct {
	params  *MLEParams
	options SimOptions
	rand    *rand.Rand
}

// NewSimulator creates a new simulation engine
func NewSimulator(params *MLEParams, options SimOptions) *Simulator {
	return &Simulator{
		params:  params,
		options: options,
		rand:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SimulateSeason runs Monte Carlo simulation for final league table
func (s *Simulator) SimulateSeason(teams []string) (map[string][]float64, map[string]float64) {
	numTeams := len(teams)
	positionCounts := make(map[string][]int)
	pointsTotals := make(map[string]float64)
	
	// Initialize counters
	for _, team := range teams {
		positionCounts[team] = make([]int, numTeams)
		pointsTotals[team] = 0.0
	}

	// Run simulation paths
	for path := 0; path < s.options.NPaths; path++ {
		// Simulate season and get final table
		seasonResults := s.simulateSeasonPath(teams)
		
		// Sort teams by points (and goal difference as tiebreaker)
		sort.Slice(seasonResults, func(i, j int) bool {
			if seasonResults[i].Points == seasonResults[j].Points {
				return seasonResults[i].GD > seasonResults[j].GD
			}
			return seasonResults[i].Points > seasonResults[j].Points
		})
		
		// Record positions and accumulate points
		for position, result := range seasonResults {
			positionCounts[result.Team][position]++
			pointsTotals[result.Team] += float64(result.Points)
		}
	}

	// Convert counts to probabilities
	positionProbs := make(map[string][]float64)
	expectedPoints := make(map[string]float64)
	
	for _, team := range teams {
		positionProbs[team] = make([]float64, numTeams)
		for pos := 0; pos < numTeams; pos++ {
			positionProbs[team][pos] = float64(positionCounts[team][pos]) / float64(s.options.NPaths)
		}
		expectedPoints[team] = pointsTotals[team] / float64(s.options.NPaths)
	}

	return positionProbs, expectedPoints
}

// simulateSeasonPath simulates one complete season path
func (s *Simulator) simulateSeasonPath(teams []string) []SimPoint {
	results := make([]SimPoint, len(teams))
	
	// Initialize team results
	for i, team := range teams {
		results[i] = SimPoint{
			Team:   team,
			Points: 0,
			GD:     0,
		}
	}

	// Simulate all matches in a round-robin tournament
	// Each team plays every other team twice (home and away)
	for i, homeTeam := range teams {
		for j, awayTeam := range teams {
			if i == j {
				continue // Team doesn't play itself
			}
			
			// Simulate the match
			homeGoals, awayGoals := s.simulateMatch(homeTeam, awayTeam)
			
			// Update points and goal difference
			results[i].GD += homeGoals - awayGoals
			results[j].GD += awayGoals - homeGoals
			
			if homeGoals > awayGoals {
				// Home team wins
				results[i].Points += 3
			} else if homeGoals < awayGoals {
				// Away team wins
				results[j].Points += 3
			} else {
				// Draw
				results[i].Points += 1
				results[j].Points += 1
			}
		}
	}

	return results
}

// simulateMatch simulates a single match outcome using Poisson distributions
func (s *Simulator) simulateMatch(homeTeam, awayTeam string) (int, int) {
	// Get team ratings
	homeAttack := s.params.AttackRatings[homeTeam]
	homeDefense := s.params.DefenseRatings[homeTeam]
	awayAttack := s.params.AttackRatings[awayTeam]
	awayDefense := s.params.DefenseRatings[awayTeam]

	// Calculate expected goals
	lambdaHome := math.Exp(homeAttack - awayDefense + s.params.HomeAdvantage)
	lambdaAway := math.Exp(awayAttack - homeDefense)

	// Add small amount of noise to prevent deterministic outcomes
	noise := 0.05
	lambdaHome *= (1.0 + noise*(s.rand.Float64()-0.5))
	lambdaAway *= (1.0 + noise*(s.rand.Float64()-0.5))

	// Ensure realistic bounds
	lambdaHome = math.Max(0.1, math.Min(lambdaHome, 5.0))
	lambdaAway = math.Max(0.1, math.Min(lambdaAway, 5.0))

	// Sample from Poisson distributions
	homeGoals := s.samplePoisson(lambdaHome)
	awayGoals := s.samplePoisson(lambdaAway)

	return homeGoals, awayGoals
}

// samplePoisson samples from a Poisson distribution with given lambda
func (s *Simulator) samplePoisson(lambda float64) int {
	if lambda < 0.1 {
		return 0
	}

	// For small lambda, use Knuth's algorithm
	if lambda < 10.0 {
		l := math.Exp(-lambda)
		k := 0
		p := 1.0

		for p > l {
			k++
			p *= s.rand.Float64()
		}

		return k - 1
	}

	// For larger lambda, use normal approximation
	return int(s.rand.NormFloat64()*math.Sqrt(lambda) + lambda + 0.5)
}

// CalculateMarketProbabilities computes probabilities for specific betting markets
func (s *Simulator) CalculateMarketProbabilities(teams []string, markets []Market) map[string]float64 {
	// Run simulation to get position probabilities
	positionProbs, _ := s.SimulateSeason(teams)
	
	marketProbs := make(map[string]float64)
	
	for _, market := range markets {
		prob := 0.0
		
		switch market.Type {
		case "winner":
			// Sum probability of finishing 1st for all teams in market
			for _, team := range market.Teams {
				if teamProbs, exists := positionProbs[team]; exists && len(teamProbs) > 0 {
					prob += teamProbs[0]
				}
			}
			
		case "top4":
			// Sum probability of finishing 1st-4th for all teams in market
			for _, team := range market.Teams {
				if teamProbs, exists := positionProbs[team]; exists && len(teamProbs) >= 4 {
					for i := 0; i < 4; i++ {
						prob += teamProbs[i]
					}
				}
			}
			
		case "relegation":
			// Sum probability of finishing in bottom 3 for all teams in market
			numPositions := len(teams)
			for _, team := range market.Teams {
				if teamProbs, exists := positionProbs[team]; exists && len(teamProbs) >= 3 {
					for i := numPositions - 3; i < numPositions; i++ {
						prob += teamProbs[i]
					}
				}
			}
		}
		
		marketProbs[market.Name] = prob
	}
	
	return marketProbs
}

// GetTeamStats returns detailed statistics for all teams
func (s *Simulator) GetTeamStats(teams []string) []TeamStats {
	positionProbs, expectedPoints := s.SimulateSeason(teams)
	numTeams := len(teams)
	
	stats := make([]TeamStats, len(teams))
	
	for i, team := range teams {
		teamProbs := positionProbs[team]
		
		stats[i] = TeamStats{
			Name:           team,
			ExpectedPoints: expectedPoints[team],
			PositionProbs:  teamProbs,
			WinnerProb:     teamProbs[0],
		}
		
		// Calculate top 4 probability
		if len(teamProbs) >= 4 {
			for j := 0; j < 4; j++ {
				stats[i].Top4Prob += teamProbs[j]
			}
		}
		
		// Calculate relegation probability (bottom 3)
		if len(teamProbs) >= 3 {
			for j := numTeams - 3; j < numTeams; j++ {
				stats[i].RelegationProb += teamProbs[j]
			}
		}
	}
	
	// Sort by expected points (descending)
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].ExpectedPoints > stats[j].ExpectedPoints
	})
	
	return stats
}