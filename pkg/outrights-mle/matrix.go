package outrightsmle

// ScoreMatrix represents the outer product of two Poisson distributions
// creating a matrix of correct score probabilities
type ScoreMatrix struct {
	HomeGoals int           // Maximum home goals (bound)
	AwayGoals int           // Maximum away goals (bound)
	Matrix    [][]float64   // [homeGoals][awayGoals] -> probability
}

// NewScoreMatrix creates a score matrix from Poisson lambdas with Dixon-Coles adjustment
func NewScoreMatrix(lambdaHome, lambdaAway, rho float64, bound int) *ScoreMatrix {
	matrix := make([][]float64, bound+1)
	for i := range matrix {
		matrix[i] = make([]float64, bound+1)
	}

	// Fill matrix with Poisson probabilities + Dixon-Coles adjustment
	for homeGoals := 0; homeGoals <= bound; homeGoals++ {
		for awayGoals := 0; awayGoals <= bound; awayGoals++ {
			probHome := PoissonProb(lambdaHome, homeGoals)
			probAway := PoissonProb(lambdaAway, awayGoals)
			
			// Apply Dixon-Coles adjustment for low-scoring games
			adjustment := DixonColesAdjustment(homeGoals, awayGoals, rho)
			
			matrix[homeGoals][awayGoals] = probHome * probAway * adjustment
		}
	}

	return &ScoreMatrix{
		HomeGoals: bound,
		AwayGoals: bound,
		Matrix:    matrix,
	}
}

// MatchOdds returns 1X2 probabilities [home_win, draw, away_win]
func (m *ScoreMatrix) MatchOdds() [3]float64 {
	var homeWin, draw, awayWin float64

	for homeGoals := 0; homeGoals <= m.HomeGoals; homeGoals++ {
		for awayGoals := 0; awayGoals <= m.AwayGoals; awayGoals++ {
			prob := m.Matrix[homeGoals][awayGoals]
			
			if homeGoals > awayGoals {
				homeWin += prob
			} else if homeGoals == awayGoals {
				draw += prob
			} else {
				awayWin += prob
			}
		}
	}

	return [3]float64{homeWin, draw, awayWin}
}

// OverUnder returns probability of total goals over/under a threshold
func (m *ScoreMatrix) OverUnder(threshold int) (over, under float64) {
	for homeGoals := 0; homeGoals <= m.HomeGoals; homeGoals++ {
		for awayGoals := 0; awayGoals <= m.AwayGoals; awayGoals++ {
			totalGoals := homeGoals + awayGoals
			prob := m.Matrix[homeGoals][awayGoals]
			
			if totalGoals > threshold {
				over += prob
			} else {
				under += prob
			}
		}
	}
	
	return over, under
}

// BothTeamsToScore returns probability of both teams scoring vs not
func (m *ScoreMatrix) BothTeamsToScore() (both, notBoth float64) {
	for homeGoals := 0; homeGoals <= m.HomeGoals; homeGoals++ {
		for awayGoals := 0; awayGoals <= m.AwayGoals; awayGoals++ {
			prob := m.Matrix[homeGoals][awayGoals]
			
			if homeGoals > 0 && awayGoals > 0 {
				both += prob
			} else {
				notBoth += prob
			}
		}
	}
	
	return both, notBoth
}

// CorrectScore returns the probability of a specific scoreline
func (m *ScoreMatrix) CorrectScore(homeGoals, awayGoals int) float64 {
	if homeGoals > m.HomeGoals || awayGoals > m.AwayGoals {
		return 0.0
	}
	return m.Matrix[homeGoals][awayGoals]
}

// ExpectedGoals returns expected home and away goals
func (m *ScoreMatrix) ExpectedGoals() (homeExpected, awayExpected float64) {
	for homeGoals := 0; homeGoals <= m.HomeGoals; homeGoals++ {
		for awayGoals := 0; awayGoals <= m.AwayGoals; awayGoals++ {
			prob := m.Matrix[homeGoals][awayGoals]
			homeExpected += float64(homeGoals) * prob
			awayExpected += float64(awayGoals) * prob
		}
	}
	
	return homeExpected, awayExpected
}

// TotalProbability returns the sum of all probabilities in the matrix
// Should be close to 1.0 (may be less if bound truncates distribution)
func (m *ScoreMatrix) TotalProbability() float64 {
	total := 0.0
	for homeGoals := 0; homeGoals <= m.HomeGoals; homeGoals++ {
		for awayGoals := 0; awayGoals <= m.AwayGoals; awayGoals++ {
			total += m.Matrix[homeGoals][awayGoals]
		}
	}
	return total
}

// DixonColesAdjustment applies the Dixon-Coles adjustment for low-scoring games
// This is now a standalone function that can be used by ScoreMatrix
func DixonColesAdjustment(homeGoals, awayGoals int, rho float64) float64 {
	// Dixon-Coles adjustment only applies to scores 0-0, 1-0, 0-1, 1-1
	switch {
	case homeGoals == 0 && awayGoals == 0:
		return 1 - rho
	case homeGoals == 1 && awayGoals == 0:
		return 1 + rho
	case homeGoals == 0 && awayGoals == 1:
		return 1 + rho
	case homeGoals == 1 && awayGoals == 1:
		return 1 - rho
	default:
		return 1.0
	}
}