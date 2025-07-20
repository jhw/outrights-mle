package outrightsmle

import "time"

// MatchResult represents a completed football match with result
type MatchResult struct {
	Date      string `json:"date"`
	Season    string `json:"season"`
	League    string `json:"league"`
	HomeTeam  string `json:"home_team"`
	AwayTeam  string `json:"away_team"`
	HomeGoals int    `json:"home_goals"`
	AwayGoals int    `json:"away_goals"`
}


// MLEParams holds the Maximum Likelihood Estimation parameters
type MLEParams struct {
	HomeAdvantage    float64            `json:"home_advantage"`    // Default: 0.3
	Rho              float64            `json:"rho"`               // Dixon-Coles parameter: -0.1
	AttackRatings    map[string]float64 `json:"attack_ratings"`
	DefenseRatings   map[string]float64 `json:"defense_ratings"`
	LogLikelihood    float64            `json:"log_likelihood"`
	Iterations       int                `json:"iterations"`
	Converged        bool               `json:"converged"`
}

// SimParams holds all simulation and MLE parameterization values
type SimParams struct {
	// Core MLE parameters
	HomeAdvantage         float64 `json:"home_advantage"`          // Home team advantage (default: 0.3)
	
	// Learning parameters
	BaseLearningRate         float64 `json:"base_learning_rate"`         // Base learning rate for gradient ascent (default: 0.001)
	LeagueChangeLearningRate float64 `json:"league_change_learning_rate"` // Enhancement multiplier for teams that changed leagues (default: 2.0)
	
	// Time weighting parameters
	TimeDecayBase         float64 `json:"time_decay_base"`         // Time decay base factor (default: 0.85)
	TimeDecayPower        float64 `json:"time_decay_power"`        // Time decay power exponent (default: 1.5)
	
	// Optimization parameters
	MaxIterations         int     `json:"max_iterations"`          // Maximum MLE iterations (default: 200)
	Tolerance             float64 `json:"tolerance"`               // Convergence tolerance (default: 1e-6)
	
	// Simulation parameters
	SimulationPaths       int     `json:"simulation_paths"`        // Monte Carlo simulation paths (default: 5000)
	GoalSimulationBound   int     `json:"goal_simulation_bound"`   // Upper bound for goal calculations (default: 5)
	GoalDifferenceEffect  float64 `json:"goal_difference_effect"`  // Goal difference multiplier in simulation (default: 0.1)
}

// MLEOptions configures the MLE optimization parameters
type MLEOptions struct {
	SimParams *SimParams `json:"sim_params,omitempty"` // Simulation parameters (uses defaults if nil)
	Debug     bool       `json:"debug"`                // Enable debug output during optimization
}


// MLEResult contains the output of MLE optimization
type MLEResult struct {
	Teams            []Team        `json:"teams"`
	MLEParams        MLEParams     `json:"mle_params"`
	ProcessingTime   time.Duration `json:"processing_time"`
	MatchesProcessed int           `json:"matches_processed"`
}

// MLERequest contains all parameters needed for MLE optimization
type MLERequest struct {
	HistoricalData []MatchResult     `json:"historical_data"`
	LeagueChangeTeams map[string]bool `json:"league_change_teams"` // Teams that changed leagues before season start
	LeagueGroups   map[string][]string `json:"league_groups,omitempty"` // Optional: league -> teams mapping
	Options        MLEOptions        `json:"options"`
}

// Team represents a team with all related parameters
type Team struct {
	Name                 string  `json:"name"`
	Points               int     `json:"points"`
	GoalDifference       int     `json:"goal_difference"`
	Played               int     `json:"played"`
	AttackRating         float64 `json:"attack_rating"`
	DefenseRating        float64 `json:"defense_rating"`
	LambdaHome           float64 `json:"lambda_home"`
	LambdaAway           float64 `json:"lambda_away"`
	ExpectedSeasonPoints float64 `json:"expected_season_points"`
}

// Event represents a match event (adapted from go-outrights)
type Event struct {
	Name  string `json:"name"`
	Date  string `json:"date"`
	Score []int  `json:"score,omitempty"`
}

// Market represents a betting market (adapted from go-outrights)
type Market struct {
	Name         string    `json:"name"`
	League       string    `json:"league"`          // League this market applies to
	Payoff       string    `json:"payoff"`          // Payoff expression like "1|4x0.25|19x0"
	ParsedPayoff []float64 `json:"-"`               // Parsed version, not serialized
	Teams        []string  `json:"teams,omitempty"` // Computed teams for this market
	Include      []string  `json:"include,omitempty"`
	Exclude      []string  `json:"exclude,omitempty"`
}



// DefaultSimParams returns default simulation and MLE parameterization values
func DefaultSimParams() *SimParams {
	return &SimParams{
		// Core MLE parameters
		HomeAdvantage:         0.3,   // Home team advantage
		
		// Learning parameters
		BaseLearningRate:         0.001,  // Base learning rate for gradient ascent
		LeagueChangeLearningRate: 2.0,    // Enhancement multiplier for teams that changed leagues
		
		// Time weighting parameters
		TimeDecayBase:        0.85,   // Time decay base factor
		TimeDecayPower:       1.5,    // Time decay power exponent
		
		// Optimization parameters
		MaxIterations:        200,    // Maximum MLE iterations
		Tolerance:            1e-6,   // Convergence tolerance
		
		// Simulation parameters
		SimulationPaths:      5000,   // Monte Carlo simulation paths
		GoalSimulationBound:  5,      // Upper bound for goal calculations
		GoalDifferenceEffect: 0.1,    // Goal difference multiplier in simulation
	}
}

// DefaultMLEOptions returns default MLE optimization options
func DefaultMLEOptions() MLEOptions {
	return MLEOptions{
		SimParams: DefaultSimParams(),
		Debug:     false,
	}
}