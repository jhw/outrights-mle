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

// MLEOptions configures the MLE optimization parameters
type MLEOptions struct {
	MaxIter     int     `json:"max_iter"`     // Maximum MLE iterations (default: 200)
	Tolerance   float64 `json:"tolerance"`    // Convergence tolerance (default: 1e-6)
	LearningRate float64 `json:"learning_rate"` // Base learning rate (default: 0.1)
	TimeDecay   float64 `json:"time_decay"`   // Time decay factor (default: 0.78)
}


// MLEResult contains the output of MLE optimization
type MLEResult struct {
	League           string        `json:"league"`
	Season           string        `json:"season"`
	TeamRatings      []TeamRating  `json:"team_ratings"`
	MLEParams        MLEParams     `json:"mle_params"`
	ProcessingTime   time.Duration `json:"processing_time"`
	MatchesProcessed int           `json:"matches_processed"`
}

// MLERequest contains all parameters needed for MLE optimization
type MLERequest struct {
	League         string        `json:"league"`
	Season         string        `json:"season"`
	HistoricalData []MatchResult `json:"historical_data"`
	Options        MLEOptions    `json:"options"`
}



// DefaultMLEOptions returns default MLE optimization options
func DefaultMLEOptions() MLEOptions {
	return MLEOptions{
		MaxIter:      200,
		Tolerance:    1e-6,
		LearningRate: 0.1,
		TimeDecay:    0.78,
	}
}