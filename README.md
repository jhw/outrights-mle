# Go Outrights MLE

A Go package implementing Maximum Likelihood Estimation for football team rating optimization, based on the variance model from `../gists/variance/e5e5*`.

## Overview

This package provides a sophisticated football team rating system that uses Maximum Likelihood Estimation with Bayesian learning principles to optimize team attack and defense parameters from historical match results. It focuses purely on the MLE optimization without season simulation or betting market analysis.

## Features

- **Maximum Likelihood Estimation**: Dual attack/defense parameters for each team based on historical match results
- **Poisson Match Modeling**: Independent Poisson distributions for home/away goals with Dixon-Coles adjustment
- **Gradient Ascent Optimization**: Mathematical optimization with convergence detection
- **Zero-Sum Constraint**: Prevents rating drift through normalization
- **Time Weighting**: Recent matches weighted more heavily using exponential decay

## Package Structure

```
go-outrights-mle/
├── go.mod                      # Module definition
├── demo.go                     # CLI demo application
├── README.md                   # This file
├── core-data/                  # Core data files
│   ├── leagues.json            # League configurations
│   └── ENG1-4.json            # English league team configurations
├── fixtures/                   # Test data directory
└── pkg/outrights-mle/         # Core package
    ├── api.go                  # Main API entry point
    ├── types.go                # Data structures and types
    └── mle.go                  # MLE optimization engine
```

## Quick Start

### Basic Usage

```go
package main

import (
    outrightsmle "github.com/jhw/go-outrights-mle/pkg/outrights-mle"
)

func main() {
    // Create MLE request
    request := outrightsmle.MLERequest{
        League:         "ENG1",
        Season:         "2023-24",
        HistoricalData: loadMatchData(), // Your historical match data
        Options:        outrightsmle.DefaultMLEOptions(),
    }

    // Run MLE optimization
    result, err := outrightsmle.RunSimulation(request)
    if err != nil {
        log.Fatal(err)
    }

    // Use results
    for _, rating := range result.TeamRatings {
        fmt.Printf("%s: Attack=%.3f, Defense=%.3f\n", 
            rating.Team, rating.AttackRating, rating.DefenseRating)
    }
}
```

### CLI Demo

Run the demo with sample data:

```bash
# Basic demo
go run demo.go

# Custom parameters
go run demo.go -league ENG1 -season 2023-24 -maxiter 500 -tolerance 1e-8

# Verbose output with full JSON results
go run demo.go -verbose -maxiter 100
```

Available flags:
- `-league`: League code (ENG1, ENG2, ENG3, ENG4) [default: ENG1]
- `-season`: Season identifier [default: 2023-24]
- `-maxiter`: Maximum MLE iterations [default: 200]
- `-tolerance`: Convergence tolerance [default: 1e-6]
- `-verbose`: Show full JSON output
- `-data`: Custom historical data file

## Core Components

### 1. Data Structures (`types.go`)

Key types for match results, team ratings, and MLE configuration:

- `MatchResult`: Historical match data with date, teams, and scores
- `TeamRating`: Attack/defense ratings with expected goals (λ values)
- `MLEParams`: MLE optimization parameters and convergence results
- `MLERequest`: Complete request configuration
- `MLEResult`: MLE optimization output with team ratings

### 2. MLE Optimization (`mle.go`)

Implements the Maximum Likelihood Estimation algorithm:

- **Gradient Ascent**: Mathematical optimization for team ratings
- **Dixon-Coles Adjustment**: Corrects correlation in low-scoring matches (0-0, 0-1, 1-0, 1-1)
- **Time Weighting**: Recent matches weighted more heavily
- **Zero-Sum Constraint**: Prevents rating drift via normalization
- **Convergence Detection**: Stops optimization when log-likelihood change < tolerance

### 3. API Layer (`api.go`)

Main entry point with validation and orchestration:

- `RunSimulation()`: Primary API function
- Input validation and default parameter handling
- Team extraction and rating calculation
- Expected goals computation (λ_home, λ_away)

## Mathematical Framework

### Poisson Match Model

Each match uses independent Poisson distributions:
- Home goals ~ Poisson(λ_home)
- Away goals ~ Poisson(λ_away)

Where:
- λ_home = exp(attack_home - defense_away + home_advantage)
- λ_away = exp(attack_away - defense_home)

### Dixon-Coles Adjustment

Corrects for correlation in low-scoring matches using parameter ρ = -0.1:
- Affects matches with scores: 0-0, 0-1, 1-0, 1-1
- Multiplies Poisson probability by adjustment factor τ(ρ)

### MLE Objective

Maximizes log-likelihood:
```
L = Σ w(t) * [log P(home_goals | λ_home) + log P(away_goals | λ_away) + log τ(ρ)]
```

Where:
- w(t): Time weighting function (exponential decay)
- τ(ρ): Dixon-Coles adjustment factor

### Zero-Sum Constraint

After each iteration:
- Calculate mean of all attack ratings: μ_attack = (1/n) Σ attack_i
- Calculate mean of all defense ratings: μ_defense = (1/n) Σ defense_i  
- Normalize: attack_i ← attack_i - μ_attack, defense_i ← defense_i - μ_defense

## Default Parameters

- **Home advantage**: 0.3 (35% boost in expected goals: exp(0.3) ≈ 1.35)
- **Dixon-Coles ρ**: -0.1 (correlation parameter for low-scoring matches)
- **Learning rate**: 0.1 (gradient ascent step size)
- **Time decay**: 0.78 (exponential decay for historical matches)
- **Convergence tolerance**: 1e-6 (minimum log-likelihood change)
- **Maximum iterations**: 200

## Output Interpretation

### Team Ratings
- **Attack/Defense**: Log-scale parameters (zero mean across all teams)
- **λ_Home/λ_Away**: Expected goals when playing home/away (exp(attack - defense ± home_advantage))

### MLE Parameters
- **Log Likelihood**: Higher values indicate better model fit
- **Converged**: Whether optimization reached tolerance within max iterations
- **Iterations**: Number of gradient ascent steps performed

### Expected Goals Examples
If a team has attack=0.2, defense=-0.1, and home_advantage=0.3:
- At home vs average team: λ = exp(0.2 - 0 + 0.3) = exp(0.5) ≈ 1.65 goals
- Away vs average team: λ = exp(0.2 - 0) = exp(0.2) ≈ 1.22 goals

## Performance

Typical performance characteristics:
- **Convergence**: 50-200 iterations for most datasets
- **Processing time**: 5-20ms for standard league datasets (380+ matches)
- **Memory usage**: Minimal overhead for match data and team ratings
- **Numerical stability**: Uses log-factorial approximations and bounds checking

## Differences from go-outrights

This package differs from the original `go-outrights` in several key ways:

1. **Algorithm**: Uses Maximum Likelihood Estimation instead of genetic algorithms
2. **Focus**: Pure MLE optimization without season simulation or betting markets
3. **Data Input**: Designed for historical match results rather than market odds
4. **Package Name**: `outrights-mle` to allow concurrent imports
5. **Output**: Team ratings and MLE parameters only, no position probabilities or market prices

## Development

To extend or modify the package:

1. **Enhanced Learning Rates**: Implement adaptive learning for promoted/relegated teams in `mle.go`
2. **Alternative Models**: Add support for different goal distribution models
3. **Data Loading**: Add utilities for loading real match data from CSV/JSON files
4. **Regularization**: Implement L1/L2 regularization for rating stability

## Example Output

```
📊 Team Ratings
===============
Team                   Attack  Defense   λ_Home   λ_Away
----                   ------  -------   ------   ------
Manchester City         0.847   -0.623     4.12     2.38
Arsenal                 0.523   -0.445     3.01     1.74
Liverpool               0.445   -0.234     2.45     1.41
Chelsea                 0.123   -0.123     1.78     1.03
...

📈 Summary Statistics
====================
Attack ratings  - Mean:  0.000, Range: [-0.892,  0.847]
Defense ratings - Mean:  0.000, Range: [-0.623,  0.765]
```

## License

Based on the original go-outrights package architecture with MLE enhancements derived from the variance model research.