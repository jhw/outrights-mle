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

// TeamRating represents attack and defense ratings for a team
type TeamRating struct {
	Team          string  `json:"team"`
	AttackRating  float64 `json:"attack_rating"`
	DefenseRating float64 `json:"defense_rating"`
	LambdaHome    float64 `json:"lambda_home"`    // Expected goals at home
	LambdaAway    float64 `json:"lambda_away"`    // Expected goals away
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

// SimOptions configures the simulation parameters
type SimOptions struct {
	NPaths      int     `json:"npaths"`       // Number of simulation paths (default: 5000)
	MaxIter     int     `json:"max_iter"`     // Maximum MLE iterations (default: 200)
	Tolerance   float64 `json:"tolerance"`    // Convergence tolerance (default: 1e-6)
	LearningRate float64 `json:"learning_rate"` // Base learning rate (default: 0.1)
	TimeDecay   float64 `json:"time_decay"`   // Time decay factor (default: 0.78)
}

// Market represents a betting market with payoff structure
type Market struct {
	Name      string   `json:"name"`
	Payoff    string   `json:"payoff"`    // e.g., "1.0|19x0"
	Teams     []string `json:"teams"`     // Teams included in market
	Type      string   `json:"type"`      // winner, top4, relegation, etc.
}

// SimulationResult contains the output of a season simulation
type SimulationResult struct {
	League            string                    `json:"league"`
	Season            string                    `json:"season"`
	TeamRatings       []TeamRating              `json:"team_ratings"`
	MLEParams         MLEParams                 `json:"mle_params"`
	ExpectedPoints    map[string]float64        `json:"expected_points"`
	PositionProbs     map[string][]float64      `json:"position_probs"`     // [team][position] = probability
	MarketPrices      map[string]float64        `json:"market_prices"`      // [market_name] = probability
	ProcessingTime    time.Duration             `json:"processing_time"`
	MatchesProcessed  int                       `json:"matches_processed"`
}

// SimulationRequest contains all parameters needed for simulation
type SimulationRequest struct {
	League      string         `json:"league"`
	Season      string         `json:"season"`
	HistoricalData []MatchResult `json:"historical_data"`
	Markets     []Market       `json:"markets"`
	Options     SimOptions     `json:"options"`
}

// TeamStats holds simulation results for a single team
type TeamStats struct {
	Name            string    `json:"name"`
	ExpectedPoints  float64   `json:"expected_points"`
	PositionProbs   []float64 `json:"position_probs"`   // Probability of finishing in each position
	WinnerProb      float64   `json:"winner_prob"`
	Top4Prob        float64   `json:"top4_prob"`
	RelegationProb  float64   `json:"relegation_prob"`
}

// SimPoint tracks points for a team across simulation paths
type SimPoint struct {
	Team   string `json:"team"`
	Points int    `json:"points"`
	GD     int    `json:"goal_difference"`
}

// DefaultSimOptions returns default simulation options
func DefaultSimOptions() SimOptions {
	return SimOptions{
		NPaths:       5000,
		MaxIter:      200,
		Tolerance:    1e-6,
		LearningRate: 0.1,
		TimeDecay:    0.78,
	}
}