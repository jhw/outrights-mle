package outrightsmle

import (
	"math"
)

// MLESolver implements Maximum Likelihood Estimation for team ratings
type MLESolver struct {
	matches  []MatchResult
	options  SimOptions
	teams    []string
	teamIdx  map[string]int
}

// NewMLESolver creates a new MLE solver instance
func NewMLESolver(matches []MatchResult, options SimOptions) *MLESolver {
	teams := extractTeams(matches)
	teamIdx := make(map[string]int)
	for i, team := range teams {
		teamIdx[team] = i
	}

	return &MLESolver{
		matches: matches,
		options: options,
		teams:   teams,
		teamIdx: teamIdx,
	}
}

// Optimize runs the MLE optimization algorithm
func (s *MLESolver) Optimize() (*MLEParams, error) {
	numTeams := len(s.teams)
	
	// Initialize parameters
	params := &MLEParams{
		HomeAdvantage:  0.3,  // Default home advantage
		Rho:           -0.1,  // Dixon-Coles parameter
		AttackRatings:  make(map[string]float64, numTeams),
		DefenseRatings: make(map[string]float64, numTeams),
	}

	// Initialize ratings to zero (average team)
	for _, team := range s.teams {
		params.AttackRatings[team] = 0.0
		params.DefenseRatings[team] = 0.0
	}

	var prevLogLikelihood float64 = -math.Inf(1)
	
	// Gradient ascent optimization
	for iter := 0; iter < s.options.MaxIter; iter++ {
		// Calculate current log likelihood
		logLikelihood := s.calculateLogLikelihood(params)
		
		// Check convergence
		if math.Abs(logLikelihood-prevLogLikelihood) < s.options.Tolerance {
			params.LogLikelihood = logLikelihood
			params.Iterations = iter + 1
			params.Converged = true
			return params, nil
		}

		// Update parameters using gradient ascent
		s.updateParameters(params)
		
		// Apply zero-sum constraint (normalize ratings)
		s.normalizeRatings(params)
		
		prevLogLikelihood = logLikelihood
	}

	// Maximum iterations reached
	params.LogLikelihood = s.calculateLogLikelihood(params)
	params.Iterations = s.options.MaxIter
	params.Converged = false

	return params, nil
}

// calculateLogLikelihood computes the log likelihood of the current parameters
func (s *MLESolver) calculateLogLikelihood(params *MLEParams) float64 {
	logLikelihood := 0.0

	for _, match := range s.matches {
		homeAttack := params.AttackRatings[match.HomeTeam]
		homeDefense := params.DefenseRatings[match.HomeTeam]
		awayAttack := params.AttackRatings[match.AwayTeam]
		awayDefense := params.DefenseRatings[match.AwayTeam]

		// Expected goals using Poisson model
		lambdaHome := math.Exp(homeAttack - awayDefense + params.HomeAdvantage)
		lambdaAway := math.Exp(awayAttack - homeDefense)

		// Poisson probabilities
		probHome := s.poissonProb(lambdaHome, match.HomeGoals)
		probAway := s.poissonProb(lambdaAway, match.AwayGoals)

		// Dixon-Coles adjustment for low-scoring games
		adjustment := s.dixonColesAdjustment(match.HomeGoals, match.AwayGoals, lambdaHome, lambdaAway, params.Rho)

		// Time weighting (more recent matches weighted higher)
		timeWeight := s.getTimeWeight(match.Season, match.Date)

		// Add to log likelihood
		matchLogLikelihood := math.Log(probHome * probAway * adjustment)
		logLikelihood += timeWeight * matchLogLikelihood
	}

	return logLikelihood
}

// updateParameters performs one step of gradient ascent
func (s *MLESolver) updateParameters(params *MLEParams) {
	// Calculate gradients for each parameter
	attackGradients := make(map[string]float64)
	defenseGradients := make(map[string]float64)

	for _, team := range s.teams {
		attackGradients[team] = 0.0
		defenseGradients[team] = 0.0
	}

	// Compute gradients from all matches
	for _, match := range s.matches {
		homeAttack := params.AttackRatings[match.HomeTeam]
		homeDefense := params.DefenseRatings[match.HomeTeam]
		awayAttack := params.AttackRatings[match.AwayTeam]
		awayDefense := params.DefenseRatings[match.AwayTeam]

		lambdaHome := math.Exp(homeAttack - awayDefense + params.HomeAdvantage)
		lambdaAway := math.Exp(awayAttack - homeDefense)

		timeWeight := s.getTimeWeight(match.Season, match.Date)
		learningRate := s.getAdaptiveLearningRate(match.HomeTeam, match.AwayTeam, match.Season)

		// Gradient contributions for home team
		attackGradients[match.HomeTeam] += timeWeight * learningRate * (float64(match.HomeGoals) - lambdaHome)
		defenseGradients[match.HomeTeam] += timeWeight * learningRate * (lambdaAway - float64(match.AwayGoals))

		// Gradient contributions for away team  
		attackGradients[match.AwayTeam] += timeWeight * learningRate * (float64(match.AwayGoals) - lambdaAway)
		defenseGradients[match.AwayTeam] += timeWeight * learningRate * (lambdaHome - float64(match.HomeGoals))
	}

	// Update parameters
	for _, team := range s.teams {
		params.AttackRatings[team] += s.options.LearningRate * attackGradients[team]
		params.DefenseRatings[team] += s.options.LearningRate * defenseGradients[team]
	}
}

// normalizeRatings applies zero-sum constraint to prevent rating drift
func (s *MLESolver) normalizeRatings(params *MLEParams) {
	var attackSum, defenseSum float64
	numTeams := float64(len(s.teams))

	// Calculate means
	for _, team := range s.teams {
		attackSum += params.AttackRatings[team]
		defenseSum += params.DefenseRatings[team]
	}

	attackMean := attackSum / numTeams
	defenseMean := defenseSum / numTeams

	// Subtract means to enforce zero-sum constraint
	for _, team := range s.teams {
		params.AttackRatings[team] -= attackMean
		params.DefenseRatings[team] -= defenseMean
	}
}

// poissonProb calculates Poisson probability P(X = k) where X ~ Poisson(lambda)
func (s *MLESolver) poissonProb(lambda float64, k int) float64 {
	if k < 0 {
		return 0.0
	}
	
	// Prevent numerical overflow
	if lambda > 10.0 {
		lambda = 10.0
	}
	
	// P(X = k) = (lambda^k * e^(-lambda)) / k!
	return math.Exp(float64(k)*math.Log(lambda) - lambda - s.logFactorial(k))
}

// dixonColesAdjustment applies correction for correlation in low-scoring matches
func (s *MLESolver) dixonColesAdjustment(homeGoals, awayGoals int, lambdaHome, lambdaAway, rho float64) float64 {
	if (homeGoals == 0 && awayGoals == 0) ||
		(homeGoals == 0 && awayGoals == 1) ||
		(homeGoals == 1 && awayGoals == 0) ||
		(homeGoals == 1 && awayGoals == 1) {
		
		tau := 1.0
		if homeGoals == 0 && awayGoals == 0 {
			tau = 1.0 - lambdaHome*lambdaAway*rho
		} else if homeGoals == 0 && awayGoals == 1 {
			tau = 1.0 + lambdaHome*rho
		} else if homeGoals == 1 && awayGoals == 0 {
			tau = 1.0 + lambdaAway*rho
		} else if homeGoals == 1 && awayGoals == 1 {
			tau = 1.0 - rho
		}
		
		return tau
	}
	
	return 1.0
}

// getTimeWeight returns temporal weighting for matches
func (s *MLESolver) getTimeWeight(season, date string) float64 {
	// Simple implementation - could be enhanced with actual date parsing
	// For now, assume more recent seasons have higher weight
	baseWeight := 1.0
	
	// Apply exponential decay based on season recency
	// This is a simplified approach - in practice would use actual dates
	return baseWeight
}

// getAdaptiveLearningRate returns enhanced learning rate for newly promoted teams
func (s *MLESolver) getAdaptiveLearningRate(homeTeam, awayTeam, season string) float64 {
	// Base learning rate
	baseRate := 1.0
	
	// In practice, this would check if teams were recently promoted
	// and apply 2-3x learning rate multiplier
	return baseRate
}

// logFactorial computes log(n!) using Stirling's approximation for large n
func (s *MLESolver) logFactorial(n int) float64 {
	if n < 2 {
		return 0.0
	}
	
	if n < 10 {
		// Direct calculation for small numbers
		result := 0.0
		for i := 2; i <= n; i++ {
			result += math.Log(float64(i))
		}
		return result
	}
	
	// Stirling's approximation for larger numbers
	nf := float64(n)
	return nf*math.Log(nf) - nf + 0.5*math.Log(2*math.Pi*nf)
}