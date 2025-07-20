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
	leagueChangeTeams map[string]bool // Teams that changed leagues before season start
	params        *MLEParams
	latestSeason  string          // Dynamically determined latest season
}

// NewMLESolver creates a new MLE solver instance
func NewMLESolver(matches []MatchResult, options MLEOptions, leagueChangeTeams map[string]bool) *MLESolver {
	teamNames := make(map[string]bool)
	for _, match := range matches {
		teamNames[match.HomeTeam] = true
		teamNames[match.AwayTeam] = true
	}

	if leagueChangeTeams == nil {
		leagueChangeTeams = make(map[string]bool)
	}

	// Find latest season dynamically
	latestSeason := findLatestSeason(matches)

	return &MLESolver{
		matches:           matches,
		options:           options,
		teamNames:         teamNames,
		leagueChangeTeams: leagueChangeTeams,
		latestSeason:      latestSeason,
	}
}

// findLatestSeason determines the latest season from match data
func findLatestSeason(matches []MatchResult) string {
	latestSeason := ""
	for _, match := range matches {
		if match.Season > latestSeason {
			latestSeason = match.Season
		}
	}
	return latestSeason
}

// Optimize runs the MLE optimization algorithm
func (s *MLESolver) Optimize() (*MLEParams, error) {
	// Get simulation parameters
	simParams := s.options.SimParams

	// Initialize parameters
	s.params = &MLEParams{
		HomeAdvantage:  simParams.HomeAdvantage,  // From SimParams
		Rho:           -0.1,                      // Dixon-Coles parameter (standard value)
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
		fmt.Printf("📅 Latest season detected: %s\n", s.latestSeason)
		if len(s.leagueChangeTeams) > 0 {
			fmt.Printf("📈 Enhanced learning enabled for %d teams with league changes\n", len(s.leagueChangeTeams))
		}
	}

	learningRate := simParams.BaseLearningRate // From SimParams
	prevLogLikelihood := s.CalculateLogLikelihood()
	
	if s.options.Debug {
		fmt.Printf("Initial log-likelihood: %.4f\n", prevLogLikelihood)
	}
	
	// Gradient ascent optimization
	for iter := 0; iter < simParams.MaxIterations; iter++ {
		s.updateRatings(learningRate)
		
		currentLogLikelihood := s.CalculateLogLikelihood()
		
		// Debug output for periodic iterations
		if s.options.Debug && iter%50 == 0 && iter > 0 {
			fmt.Printf("Iteration %d: log-likelihood = %.4f (change: %.6f)\n", iter, currentLogLikelihood, currentLogLikelihood-prevLogLikelihood)
		}
		
		// Check convergence
		if iter > 0 && math.Abs(currentLogLikelihood-prevLogLikelihood) < simParams.Tolerance {
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
	s.params.Iterations = simParams.MaxIterations
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
	// Get simulation parameters
	simParams := s.options.SimParams

	// Calculate years since latest season in the data
	latestYear, err := convertSeasonToYear(s.latestSeason)
	if err != nil {
		// Log error and return no decay (weight = 1.0) as fallback
		if s.options.Debug {
			fmt.Printf("⚠️  Error parsing latest season %q: %v, using weight 1.0\n", s.latestSeason, err)
		}
		return 1.0
	}
	
	seasonYear, err := convertSeasonToYear(season)
	if err != nil {
		// Log error and return no decay (weight = 1.0) as fallback  
		if s.options.Debug {
			fmt.Printf("⚠️  Error parsing season %q: %v, using weight 1.0\n", season, err)
		}
		return 1.0
	}
	
	yearsAgo := float64(latestYear - seasonYear)
	
	// Apply exponential decay with configurable base and power
	return math.Pow(simParams.TimeDecayBase, yearsAgo*simParams.TimeDecayPower)
}


// getAdaptiveLearningRate returns enhanced learning rate for teams with league changes  
func (s *MLESolver) getAdaptiveLearningRate(team string, baseLearningRate float64, match MatchResult) float64 {
	// Get simulation parameters
	simParams := s.options.SimParams

	// Apply enhanced learning ONLY for teams in their first season after changing leagues
	if s.leagueChangeTeams[team] && match.Season == s.latestSeason {
		// Linear decay from LeagueChangeLearningRate to 1.0 over their first season in new league
		// Using time weight as proxy for "how far into season"
		enhancementRange := simParams.LeagueChangeLearningRate - 1.0
		enhancementFactor := simParams.LeagueChangeLearningRate - enhancementRange*s.getTimeWeight(s.latestSeason)
		return baseLearningRate * enhancementFactor
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
	
	// Get simulation parameters for goal simulation bound
	simParams := s.options.SimParams

	// Sum probabilities for all possible score combinations
	for homeGoals := 0; homeGoals <= simParams.GoalSimulationBound; homeGoals++ {
		for awayGoals := 0; awayGoals <= simParams.GoalSimulationBound; awayGoals++ {
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