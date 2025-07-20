package outrightsmle

import (
	"fmt"
	"math"
)

// MLESolver implements Maximum Likelihood Estimation for team ratings
type MLESolver struct {
	matches       []MatchResult
	options       MLEOptions
	teamNames     map[string]bool
	promotedTeams map[string]bool // Teams with historical league changes
	params        *MLEParams
}

// NewMLESolver creates a new MLE solver instance
func NewMLESolver(matches []MatchResult, options MLEOptions, promotedTeams map[string]bool) *MLESolver {
	teamNames := make(map[string]bool)
	for _, match := range matches {
		teamNames[match.HomeTeam] = true
		teamNames[match.AwayTeam] = true
	}

	if promotedTeams == nil {
		promotedTeams = make(map[string]bool)
	}

	return &MLESolver{
		matches:       matches,
		options:       options,
		teamNames:     teamNames,
		promotedTeams: promotedTeams,
	}
}

// Optimize runs the MLE optimization algorithm
func (s *MLESolver) Optimize() (*MLEParams, error) {
	// Initialize parameters
	s.params = &MLEParams{
		HomeAdvantage:  0.3,  // Default home advantage
		Rho:           -0.1,  // Dixon-Coles parameter
		AttackRatings:  make(map[string]float64),
		DefenseRatings: make(map[string]float64),
	}

	// Initialize ratings to zero (average team)
	for team := range s.teamNames {
		s.params.AttackRatings[team] = 0.0
		s.params.DefenseRatings[team] = 0.0
	}

	if s.options.Debug {
		fmt.Printf("🔧 Starting MLE optimization for %d teams, %d matches...\n", len(s.teamNames), len(s.matches))
		if len(s.promotedTeams) > 0 {
			fmt.Printf("📈 Enhanced learning enabled for %d teams with league changes\n", len(s.promotedTeams))
		}
	}

	learningRate := 0.001 // Match gist exactly
	prevLogLikelihood := s.CalculateLogLikelihood()
	
	if s.options.Debug {
		fmt.Printf("Initial log-likelihood: %.4f\n", prevLogLikelihood)
	}
	
	// Gradient ascent optimization
	for iter := 0; iter < s.options.MaxIter; iter++ {
		s.updateRatings(learningRate)
		
		currentLogLikelihood := s.CalculateLogLikelihood()
		
		// Debug output for periodic iterations
		if s.options.Debug && iter%50 == 0 && iter > 0 {
			fmt.Printf("Iteration %d: log-likelihood = %.4f (change: %.6f)\n", iter, currentLogLikelihood, currentLogLikelihood-prevLogLikelihood)
		}
		
		// Check convergence
		if iter > 0 && math.Abs(currentLogLikelihood-prevLogLikelihood) < s.options.Tolerance {
			s.params.LogLikelihood = currentLogLikelihood
			s.params.Iterations = iter + 1
			s.params.Converged = true
			if s.options.Debug {
				fmt.Printf("✅ Converged at iteration %d (change: %.2e)\n", iter, math.Abs(currentLogLikelihood-prevLogLikelihood))
			}
			return s.params, nil
		}
		
		prevLogLikelihood = currentLogLikelihood
	}

	// Maximum iterations reached
	s.params.LogLikelihood = s.CalculateLogLikelihood()
	s.params.Iterations = s.options.MaxIter
	s.params.Converged = false

	return s.params, nil
}

// CalculateLogLikelihood computes the log likelihood of the current parameters
func (s *MLESolver) CalculateLogLikelihood() float64 {
	logLikelihood := 0.0
	
	for _, match := range s.matches {
		homeAttack := s.params.AttackRatings[match.HomeTeam]
		homeDefense := s.params.DefenseRatings[match.HomeTeam]
		awayAttack := s.params.AttackRatings[match.AwayTeam]
		awayDefense := s.params.DefenseRatings[match.AwayTeam]
		
		lambdaHome := math.Exp(homeAttack - awayDefense + s.params.HomeAdvantage)
		lambdaAway := math.Exp(awayAttack - homeDefense)
		
		probHome := s.PoissonProb(lambdaHome, match.HomeGoals)
		probAway := s.PoissonProb(lambdaAway, match.AwayGoals)
		
		adjustment := s.DixonColesAdjustment(match.HomeGoals, match.AwayGoals, s.params.Rho)
		
		prob := probHome * probAway * adjustment
		if prob > 0 {
			// Apply time weighting to log-likelihood
			timeWeight := s.getTimeWeight(match.Season)
			logLikelihood += timeWeight * math.Log(prob)
		}
	}
	
	return logLikelihood
}

// updateRatings performs one step of gradient ascent
func (s *MLESolver) updateRatings(learningRate float64) {
	gradients := make(map[string]float64)
	teamLastMatch := make(map[string]MatchResult) // Track last match per team for adaptive LR
	
	// Calculate gradients with time weighting
	for _, match := range s.matches {
		homeAttack := s.params.AttackRatings[match.HomeTeam]
		homeDefense := s.params.DefenseRatings[match.HomeTeam]
		awayAttack := s.params.AttackRatings[match.AwayTeam]
		awayDefense := s.params.DefenseRatings[match.AwayTeam]
		
		lambdaHome := math.Exp(homeAttack - awayDefense + s.params.HomeAdvantage)
		lambdaAway := math.Exp(awayAttack - homeDefense)
		
		// Apply time weighting - recent matches matter more
		timeWeight := s.getTimeWeight(match.Season)
		
		// Gradient for home team attack
		gradients[match.HomeTeam+"_attack"] += timeWeight * (float64(match.HomeGoals) - lambdaHome)
		
		// Gradient for away team attack
		gradients[match.AwayTeam+"_attack"] += timeWeight * (float64(match.AwayGoals) - lambdaAway)
		
		// Gradient for home team defense
		gradients[match.HomeTeam+"_defense"] += timeWeight * (lambdaAway - float64(match.AwayGoals))
		
		// Gradient for away team defense  
		gradients[match.AwayTeam+"_defense"] += timeWeight * (lambdaHome - float64(match.HomeGoals))
		
		// Track most recent match for each team (for adaptive learning rate)
		teamLastMatch[match.HomeTeam] = match
		teamLastMatch[match.AwayTeam] = match
	}
	
	// Update parameters with adaptive learning rates
	for team := range s.teamNames {
		if grad, exists := gradients[team+"_attack"]; exists {
			lastMatch := teamLastMatch[team]
			adaptiveLR := s.getAdaptiveLearningRate(team, learningRate, lastMatch)
			s.params.AttackRatings[team] += adaptiveLR * grad
		}
		if grad, exists := gradients[team+"_defense"]; exists {
			lastMatch := teamLastMatch[team]
			adaptiveLR := s.getAdaptiveLearningRate(team, learningRate, lastMatch)
			s.params.DefenseRatings[team] += adaptiveLR * grad
		}
	}
	
	// Apply zero-sum constraint to prevent rating drift
	s.normalizeRatings()
}

// normalizeRatings applies zero-sum constraint to prevent rating drift
func (s *MLESolver) normalizeRatings() {
	// Calculate sums
	attackSum := 0.0
	defenseSum := 0.0
	teamCount := float64(len(s.teamNames))
	
	for team := range s.teamNames {
		attackSum += s.params.AttackRatings[team]
		defenseSum += s.params.DefenseRatings[team]
	}
	
	// Calculate averages
	attackAverage := attackSum / teamCount
	defenseAverage := defenseSum / teamCount
	
	// Subtract averages to enforce zero-sum constraint
	for team := range s.teamNames {
		s.params.AttackRatings[team] -= attackAverage
		s.params.DefenseRatings[team] -= defenseAverage
	}
}

// PoissonProb calculates Poisson probability P(X = k) where X ~ Poisson(lambda)
func (s *MLESolver) PoissonProb(lambda float64, k int) float64 {
	if k < 0 {
		return 0
	}
	if lambda <= 0 {
		if k == 0 {
			return 1.0
		}
		return 0
	}
	
	// Use log space for numerical stability
	logProb := float64(k)*math.Log(lambda) - lambda - s.logFactorial(k)
	return math.Exp(logProb)
}

// logFactorial computes log(n!) for Poisson calculations
func (s *MLESolver) logFactorial(n int) float64 {
	if n <= 1 {
		return 0
	}
	result := 0.0
	for i := 2; i <= n; i++ {
		result += math.Log(float64(i))
	}
	return result
}

// DixonColesAdjustment applies correction for correlation in low-scoring matches
func (s *MLESolver) DixonColesAdjustment(homeGoals, awayGoals int, rho float64) float64 {
	if homeGoals > 1 || awayGoals > 1 {
		return 1.0
	}
	
	switch {
	case homeGoals == 0 && awayGoals == 0:
		return 1 - rho
	case homeGoals == 0 && awayGoals == 1:
		return 1 + rho
	case homeGoals == 1 && awayGoals == 0:
		return 1 + rho
	case homeGoals == 1 && awayGoals == 1:
		return 1 - rho
	default:
		return 1.0
	}
}

// getTimeWeight returns temporal weighting for matches
func (s *MLESolver) getTimeWeight(season string) float64 {
	// Calculate years since current season (2024-25)
	currentYear := 2024
	seasonYear := s.convertSeasonToYear(season)
	yearsAgo := float64(currentYear - seasonYear)
	
	// Apply exponential decay with power of 1.5
	// 0 years ago (current): 1.0
	// 1 year ago: 0.78 (0.85^1.5)  
	// 2 years ago: 0.59 (0.85^3.0)
	return math.Pow(0.85, yearsAgo*1.5)
}

// convertSeasonToYear converts season string to starting year
func (s *MLESolver) convertSeasonToYear(season string) int {
	// Convert season string to starting year (e.g., "2425" -> 2024, "2324" -> 2023)
	if len(season) != 4 {
		return 2014 // Default to oldest season
	}
	
	// Parse first two digits and add 2000
	year := 0
	if season[0] >= '0' && season[0] <= '9' {
		year += int(season[0]-'0') * 10
	}
	if season[1] >= '0' && season[1] <= '9' {
		year += int(season[1] - '0')
	}
	
	return 2000 + year
}

// getAdaptiveLearningRate returns enhanced learning rate for teams with league changes  
func (s *MLESolver) getAdaptiveLearningRate(team string, baseLearningRate float64, match MatchResult) float64 {
	// Copy the exact gist implementation
	if s.promotedTeams[team] {
		// Decay the enhancement over the current season (2425)
		// Start with 3x rate, decay to 1x rate over the season
		if match.Season == "2425" {
			// Enhanced rate decays from 3.0 to 1.0 over current season
			// Using time weight as proxy for "how far into season"
			enhancementFactor := 3.0 - 2.0*s.getTimeWeight("2425") // 3.0 → 1.0
			return baseLearningRate * enhancementFactor
		} else {
			// For historical seasons, use moderate 2x enhancement
			return baseLearningRate * 2.0
		}
	}
	return baseLearningRate
}

// calculateExpectedMatchPoints calculates expected points for home and away teams in a match
// Copied exactly from gist lines 622-658
func (s *MLESolver) calculateExpectedMatchPoints(homeTeam, awayTeam string) (float64, float64) {
	homeAttack := s.params.AttackRatings[homeTeam]
	homeDefense := s.params.DefenseRatings[homeTeam]
	awayAttack := s.params.AttackRatings[awayTeam]
	awayDefense := s.params.DefenseRatings[awayTeam]
	
	lambdaHome := math.Exp(homeAttack - awayDefense + s.params.HomeAdvantage)
	lambdaAway := math.Exp(awayAttack - homeDefense)
	
	// Calculate probabilities for different outcomes
	var homeWinProb, drawProb, awayWinProb float64
	
	// Sum probabilities for all possible score combinations
	for homeGoals := 0; homeGoals <= 5; homeGoals++ {
		for awayGoals := 0; awayGoals <= 5; awayGoals++ {
			probHome := s.PoissonProb(lambdaHome, homeGoals)
			probAway := s.PoissonProb(lambdaAway, awayGoals)
			adjustment := s.DixonColesAdjustment(homeGoals, awayGoals, s.params.Rho)
			
			matchProb := probHome * probAway * adjustment
			
			if homeGoals > awayGoals {
				homeWinProb += matchProb
			} else if homeGoals == awayGoals {
				drawProb += matchProb
			} else {
				awayWinProb += matchProb
			}
		}
	}
	
	// Calculate expected points (3 for win, 1 for draw, 0 for loss)
	homeExpectedPoints := 3*homeWinProb + 1*drawProb
	awayExpectedPoints := 3*awayWinProb + 1*drawProb
	
	return homeExpectedPoints, awayExpectedPoints
}