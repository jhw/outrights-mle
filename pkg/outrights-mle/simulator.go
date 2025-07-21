package outrightsmle

import (
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"
)


type SimPoints struct {
	NPaths         int
	TeamNames      []string
	Points         [][]int  // Match points (3/1/0) per team per simulation path
	GoalDifference [][]int  // Goal difference per team per simulation path
	// Cache for position probabilities to avoid expensive recalculations
	positionCache map[string]map[string][]float64 // sortedTeamsKey -> teamName -> probabilities
}

func newSimPoints(teamNames []string, nPaths int) *SimPoints {
	sp := &SimPoints{
		NPaths:         nPaths,
		TeamNames:      make([]string, len(teamNames)),
		Points:         make([][]int, len(teamNames)),
		GoalDifference: make([][]int, len(teamNames)),
		positionCache:  make(map[string]map[string][]float64),
	}
	
	for i, teamName := range teamNames {
		sp.TeamNames[i] = teamName
		sp.Points[i] = make([]int, nPaths)
		sp.GoalDifference[i] = make([]int, nPaths)
		
		// Initialize all paths to 0
		for j := 0; j < nPaths; j++ {
			sp.Points[i][j] = 0
			sp.GoalDifference[i][j] = 0
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
		homeGoals := PoissonSample(lambdaHome)
		awayGoals := PoissonSample(lambdaAway)
		
		// Calculate points and goal difference
		var homePoints, awayPoints int
		if homeGoals > awayGoals {
			homePoints = 3
			awayPoints = 0
		} else if homeGoals == awayGoals {
			homePoints = 1
			awayPoints = 1
		} else {
			homePoints = 0
			awayPoints = 3
		}
		
		// Track points and goal difference separately
		homeGD := homeGoals - awayGoals
		awayGD := awayGoals - homeGoals
		
		// Add match points (3/1/0 only)
		sp.Points[homeIdx][path] += homePoints
		sp.Points[awayIdx][path] += awayPoints
		
		// Track goal difference separately for tiebreaking
		sp.GoalDifference[homeIdx][path] += homeGD
		sp.GoalDifference[awayIdx][path] += awayGD
	}
}


// positionProbabilities calculates position probabilities for given teams with caching
func (sp *SimPoints) positionProbabilities(teamNames []string) map[string][]float64 {
	if teamNames == nil {
		teamNames = sp.TeamNames
	}
	
	// Create cache key from sorted team names
	sortedNames := make([]string, len(teamNames))
	copy(sortedNames, teamNames)
	sort.Strings(sortedNames)
	cacheKey := strings.Join(sortedNames, "|")
	
	// Check cache first
	if cachedResult, exists := sp.positionCache[cacheKey]; exists {
		return cachedResult
	}
	
	// Create mask for selected teams
	selectedIndices := make([]int, 0, len(teamNames))
	for _, name := range teamNames {
		if idx := sp.getTeamIndex(name); idx >= 0 {
			selectedIndices = append(selectedIndices, idx)
		}
	}
	
	if len(selectedIndices) == 0 {
		return make(map[string][]float64)
	}
	
	// Extract points and goal difference for selected teams
	selectedPoints := make([][]int, len(selectedIndices))
	selectedGoalDiff := make([][]int, len(selectedIndices))
	for i, idx := range selectedIndices {
		selectedPoints[i] = sp.Points[idx]
		selectedGoalDiff[i] = sp.GoalDifference[idx]
	}
	
	// Calculate positions for each path
	positions := make([][]int, len(selectedIndices))
	for i := range positions {
		positions[i] = make([]int, sp.NPaths)
	}
	
	for path := 0; path < sp.NPaths; path++ {
		// Create array of team data for this path
		teamData := make([]struct {
			TeamIndex      int
			Points         float64
			GoalDifference float64
		}, len(selectedIndices))
		
		for i := range selectedIndices {
			teamData[i] = struct {
				TeamIndex      int
				Points         float64
				GoalDifference float64
			}{
				TeamIndex:      i,
				Points:         float64(selectedPoints[i][path]),
				GoalDifference: float64(selectedGoalDiff[i][path]),
			}
		}
		
		// Sort by points (descending), with goal difference as tiny tiebreaker
		// Goal difference tiebreaker factor: 0.0001 (small enough to never exceed 1 point)
		sort.Slice(teamData, func(i, j int) bool {
			teamI := teamData[i]
			teamJ := teamData[j]
			
			// Primary: sort by points (descending)
			if teamI.Points != teamJ.Points {
				return teamI.Points > teamJ.Points
			}
			
			// Tiebreaker: sort by goal difference (descending) with tiny factor
			return teamI.GoalDifference > teamJ.GoalDifference
		})
		
		// Assign positions (0 = first place, 1 = second place, etc.)
		for pos, team := range teamData {
			positions[team.TeamIndex][path] = pos
		}
	}
	
	// Calculate probabilities
	probabilities := make(map[string][]float64)
	for _, name := range teamNames {
		if idx := sp.getTeamIndex(name); idx >= 0 {
			probs := make([]float64, len(selectedIndices))
			
			// Find which index in selectedIndices this team corresponds to
			selectedIdx := -1
			for j, selIdx := range selectedIndices {
				if selIdx == idx {
					selectedIdx = j
					break
				}
			}
			
			if selectedIdx >= 0 {
				// Count occurrences of each position
				for path := 0; path < sp.NPaths; path++ {
					pos := positions[selectedIdx][path]
					probs[pos] += 1.0 / float64(sp.NPaths)
				}
			}
			
			probabilities[name] = probs
		}
	}
	
	// Cache the result
	sp.positionCache[cacheKey] = probabilities
	
	return probabilities
}

func init() {
	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())
}