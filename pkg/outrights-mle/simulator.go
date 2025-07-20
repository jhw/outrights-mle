package outrightsmle

import (
	"math"
	"math/rand"
	"time"
)

// Note: GDMultiplier moved to SimParams.GoalDifferenceEffect

type SimPoints struct {
	NPaths    int
	TeamNames []string
	Points    [][]float64
}

func newSimPoints(teamNames []string, nPaths int) *SimPoints {
	sp := &SimPoints{
		NPaths:    nPaths,
		TeamNames: make([]string, len(teamNames)),
		Points:    make([][]float64, len(teamNames)),
	}
	
	for i, teamName := range teamNames {
		sp.TeamNames[i] = teamName
		sp.Points[i] = make([]float64, nPaths)
		
		// Initialize all paths to 0 points
		for j := 0; j < nPaths; j++ {
			sp.Points[i][j] = 0.0
		}
	}
	
	return sp
}


func (sp *SimPoints) getTeamIndex(teamName string) int {
	for i, name := range sp.TeamNames {
		if name == teamName {
			return i
		}
	}
	return -1
}

// simulate simulates a single match between home and away teams across all paths
// Copied exactly from gist simulator.go lines 51-94
func (sp *SimPoints) simulate(homeTeam, awayTeam string, solver *MLESolver) {
	homeIdx := sp.getTeamIndex(homeTeam)
	awayIdx := sp.getTeamIndex(awayTeam)
	
	if homeIdx == -1 || awayIdx == -1 {
		return
	}
	
	// Get team ratings
	homeAttack := solver.params.AttackRatings[homeTeam]
	homeDefense := solver.params.DefenseRatings[homeTeam]
	awayAttack := solver.params.AttackRatings[awayTeam]
	awayDefense := solver.params.DefenseRatings[awayTeam]
	
	lambdaHome := math.Exp(homeAttack - awayDefense + solver.params.HomeAdvantage)
	lambdaAway := math.Exp(awayAttack - homeDefense)
	
	// Simulate NPaths matches
	for path := 0; path < sp.NPaths; path++ {
		// Generate Poisson scores
		homeGoals := poissonSample(lambdaHome)
		awayGoals := poissonSample(lambdaAway)
		
		// Calculate points and goal difference
		var homePoints, awayPoints float64
		if homeGoals > awayGoals {
			homePoints = 3.0
			awayPoints = 0.0
		} else if homeGoals == awayGoals {
			homePoints = 1.0
			awayPoints = 1.0
		} else {
			homePoints = 0.0
			awayPoints = 3.0
		}
		
		// Add goal difference effect using SimParams
		simParams := solver.options.SimParams
		if simParams == nil {
			simParams = DefaultSimParams()
		}
		
		homeGD := float64(homeGoals - awayGoals)
		awayGD := float64(awayGoals - homeGoals)
		
		sp.Points[homeIdx][path] += homePoints + simParams.GoalDifferenceEffect*homeGD
		sp.Points[awayIdx][path] += awayPoints + simParams.GoalDifferenceEffect*awayGD
	}
}

// poissonSample generates a random sample from Poisson distribution
// Copied exactly from gist simulator.go lines 184-204
func poissonSample(lambda float64) int {
	if lambda < 0 {
		return 0
	}
	
	// Use inverse transform sampling for small lambda
	if lambda < 12 {
		L := math.Exp(-lambda)
		k := 0
		p := 1.0
		
		for p > L {
			k++
			p *= rand.Float64()
		}
		return k - 1
	}
	
	// Use normal approximation for large lambda
	return int(math.Max(0, rand.NormFloat64()*math.Sqrt(lambda)+lambda+0.5))
}

func init() {
	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())
}