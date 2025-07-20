# Go Outrights MLE

A Go package implementing Maximum Likelihood Estimation for football prediction and betting market analysis, based on the variance model from `../gists/variance/e5e5*`.

## Overview

This package provides a sophisticated football team rating system that uses Maximum Likelihood Estimation with Bayesian learning principles to predict team strengths and season outcomes. It's designed as an MLE-based alternative to the genetic algorithm approach used in the original `go-outrights` package.

## Features

- **Maximum Likelihood Estimation**: Dual attack/defense parameters for each team based on historical match results
- **Poisson Match Modeling**: Independent Poisson distributions for home/away goals with Dixon-Coles adjustment
- **Monte Carlo Simulation**: Simulates complete seasons to predict final league tables
- **Betting Market Analysis**: Calculates fair values for various betting markets (Winner, Top 4, Relegation)
- **Bayesian Learning**: Adaptive learning rates with time weighting and promotion/relegation detection

## Package Structure

```
go-outrights-mle/
├── go.mod                      # Module definition
├── demo.go                     # CLI demo application
├── README.md                   # This file
├── core-data/                  # Core data files
│   ├── leagues.json            # League configurations
│   ├── ENG1.json              # Premier League teams/markets
│   ├── ENG2.json              # Championship teams/markets
│   ├── ENG3.json              # League One teams/markets
│   └── ENG4.json              # League Two teams/markets
├── fixtures/                   # Test data directory
└── pkg/outrights-mle/         # Core package
    ├── api.go                  # Main API entry point
    ├── types.go                # Data structures and types
    ├── mle.go                  # MLE optimization engine
    └── simulator.go            # Monte Carlo simulation
```

## Quick Start

### Basic Usage

```go
package main

import (
    outrightsmle "github.com/jhw/go-outrights-mle/pkg/outrights-mle"
)

func main() {
    // Create simulation request
    request := outrightsmle.SimulationRequest{
        League:         "ENG1",
        Season:         "2023-24",
        HistoricalData: loadMatchData(), // Your historical match data
        Markets:        loadMarkets(),   // Betting markets
        Options:        outrightsmle.DefaultSimOptions(),
    }

    // Run simulation
    result, err := outrightsmle.Simulate(request)
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
go run demo.go -league ENG1 -season 2023-24 -npaths 10000 -maxiter 500

# Verbose output with full JSON results
go run demo.go -verbose -npaths 1000
```

Available flags:
- `-league`: League code (ENG1, ENG2, ENG3, ENG4) [default: ENG1]
- `-season`: Season identifier [default: 2023-24]
- `-npaths`: Number of Monte Carlo paths [default: 5000]
- `-maxiter`: Maximum MLE iterations [default: 200]
- `-tolerance`: Convergence tolerance [default: 1e-6]
- `-verbose`: Show full JSON output
- `-data`: Custom historical data file
- `-teams`: Custom teams configuration file
- `-markets`: Custom markets configuration file

## Core Components

### 1. Data Structures (`types.go`)

Key types for match results, team ratings, and simulation configuration:

- `MatchResult`: Historical match data with date, teams, and scores
- `TeamRating`: Attack/defense ratings with expected goals
- `MLEParams`: MLE optimization parameters and results
- `SimulationRequest`: Complete request configuration
- `SimulationResult`: Simulation output with predictions and market prices

### 2. MLE Optimization (`mle.go`)

Implements the Maximum Likelihood Estimation algorithm:

- **Gradient Ascent**: Mathematical optimization for team ratings
- **Dixon-Coles Adjustment**: Corrects correlation in low-scoring matches
- **Time Weighting**: Recent matches weighted more heavily
- **Zero-Sum Constraint**: Prevents rating drift via normalization
- **Adaptive Learning**: Enhanced rates for promoted/relegated teams

### 3. Monte Carlo Simulation (`simulator.go`)

Runs season simulations using optimized team ratings:

- **Poisson Sampling**: Generates match outcomes using Poisson distributions
- **Full Season Simulation**: Complete round-robin tournament simulation
- **Position Probabilities**: Calculates finish position probabilities for each team
- **Market Pricing**: Computes fair values for betting markets

### 4. API Layer (`api.go`)

Main entry point with validation and orchestration:

- `Simulate()`: Primary API function
- Input validation and default parameter handling
- Integration of MLE optimization and Monte Carlo simulation
- Market price calculations

## Mathematical Framework

### Poisson Match Model

Each match uses independent Poisson distributions:
- Home goals ~ Poisson(λ_home)
- Away goals ~ Poisson(λ_away)

Where:
- λ_home = exp(attack_home - defense_away + home_advantage)
- λ_away = exp(attack_away - defense_home)

### Dixon-Coles Adjustment

Corrects for correlation in low-scoring matches (0-0, 0-1, 1-0, 1-1) using parameter ρ = -0.1.

### MLE Objective

Maximizes log-likelihood:
```
L = Σ w(t) * [log P(home_goals | λ_home) + log P(away_goals | λ_away) + log τ(ρ)]
```

Where:
- w(t): Time weighting function
- τ(ρ): Dixon-Coles adjustment factor

## Configuration

### Default Parameters

- Home advantage: 0.3 (35% boost in expected goals)
- Dixon-Coles ρ: -0.1
- Learning rate: 0.1
- Time decay: 0.78 (exponential decay for historical seasons)
- Convergence tolerance: 1e-6
- Monte Carlo paths: 5000

### Core Data Files

The package includes configuration files copied from `../dsol-outrights-sst/config/core-data`:

- `leagues.json`: League metadata
- `ENG1-4.json`: Team and market configurations for English leagues

## Performance

Typical performance characteristics:
- MLE convergence: 100-200 iterations
- Processing time: 10-50ms for standard datasets
- Memory usage: Minimal overhead for match data and team ratings

## Differences from go-outrights

This package differs from the original `go-outrights` in several key ways:

1. **Algorithm**: Uses Maximum Likelihood Estimation instead of genetic algorithms
2. **Data Input**: Designed for historical match results rather than market odds
3. **Package Name**: `outrights-mle` to allow concurrent imports
4. **Mathematical Framework**: Poisson-based modeling with Dixon-Coles adjustments
5. **Learning Approach**: Bayesian learning with adaptive rates for league changes

## Development

To extend or modify the package:

1. **Add New Leagues**: Create configuration files in `core-data/`
2. **Enhance MLE**: Modify algorithms in `mle.go`
3. **New Market Types**: Extend market calculations in `api.go` and `simulator.go`
4. **Data Sources**: Add data loading utilities for different formats

## License

Based on the original go-outrights package architecture with MLE enhancements derived from the variance model research.