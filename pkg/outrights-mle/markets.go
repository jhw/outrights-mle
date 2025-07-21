package outrightsmle

import (
	"fmt"
	"strconv"
	"strings"
)


// parsePayoff parses payoff expressions like "1|4x0.25|19x0" meaning 1 winner gets 1, 4 get 0.25, 19 losers get 0
// Adapted from go-outrights/pkg/outrights/markets.go
func parsePayoff(payoffExpr string) ([]float64, error) {
	var payoff []float64
	
	for _, expr := range strings.Split(payoffExpr, "|") {
		tokens := strings.Split(expr, "x")
		
		var n int
		var v float64
		var err error
		
		if len(tokens) == 1 {
			// Single value, assume n=1
			n = 1
			v, err = strconv.ParseFloat(tokens[0], 64)
		} else if len(tokens) == 2 {
			// n and value
			var err1 error
			n, err1 = strconv.Atoi(tokens[0])
			v, err = strconv.ParseFloat(tokens[1], 64)
			if err1 != nil || err != nil {
				return nil, fmt.Errorf("invalid payoff format: %s", expr)
			}
		} else {
			return nil, fmt.Errorf("invalid payoff format: %s", expr)
		}
		
		if err != nil {
			return nil, fmt.Errorf("invalid payoff format: %s", expr)
		}
		
		for i := 0; i < n; i++ {
			payoff = append(payoff, v)
		}
	}
	
	return payoff, nil
}

// initIncludeMarket initializes a market with specific included teams
// Adapted from go-outrights/pkg/outrights/markets.go
func initIncludeMarket(teamNames []string, market *Market) error {
	// Check for unknown teams
	for _, teamName := range market.Include {
		found := false
		for _, knownTeam := range teamNames {
			if teamName == knownTeam {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%s market has unknown team %s in league %s", market.Name, teamName, market.League)
		}
	}
	
	market.Teams = make([]string, len(market.Include))
	copy(market.Teams, market.Include)
	
	// Parse and validate payoff
	if market.Payoff == "" {
		return fmt.Errorf("market %s has no payoff defined", market.Name)
	}
	
	parsedPayoff, err := parsePayoff(market.Payoff)
	if err != nil {
		return fmt.Errorf("error parsing payoff for market %s: %v", market.Name, err)
	}
	market.ParsedPayoff = parsedPayoff
	
	// Validate payoff length matches include teams count
	if len(market.ParsedPayoff) != len(market.Include) {
		return fmt.Errorf("%s include market payoff length (%d) does not match include teams count (%d)", 
			market.Name, len(market.ParsedPayoff), len(market.Include))
	}
	
	return nil
}

// initExcludeMarket initializes a market excluding specific teams
// Adapted from go-outrights/pkg/outrights/markets.go
func initExcludeMarket(teamNames []string, market *Market) error {
	// Check for unknown teams
	for _, teamName := range market.Exclude {
		found := false
		for _, knownTeam := range teamNames {
			if teamName == knownTeam {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%s market has unknown team %s in league %s", market.Name, teamName, market.League)
		}
	}
	
	// Include all teams except excluded ones
	market.Teams = []string{}
	for _, teamName := range teamNames {
		excluded := false
		for _, excludedTeam := range market.Exclude {
			if teamName == excludedTeam {
				excluded = true
				break
			}
		}
		if !excluded {
			market.Teams = append(market.Teams, teamName)
		}
	}
	
	// Parse and validate payoff
	if market.Payoff == "" {
		return fmt.Errorf("market %s has no payoff defined", market.Name)
	}
	
	parsedPayoff, err := parsePayoff(market.Payoff)
	if err != nil {
		return fmt.Errorf("error parsing payoff for market %s: %v", market.Name, err)
	}
	market.ParsedPayoff = parsedPayoff
	
	// Validate payoff length matches remaining teams count (total - excluded)
	expectedLength := len(teamNames) - len(market.Exclude)
	if len(market.ParsedPayoff) != expectedLength {
		return fmt.Errorf("%s exclude market payoff length (%d) does not match remaining teams count (%d)", 
			market.Name, len(market.ParsedPayoff), expectedLength)
	}
	
	return nil
}

// initStandardMarket initializes a market with all teams
// Adapted from go-outrights/pkg/outrights/markets.go
func initStandardMarket(teamNames []string, market *Market) error {
	market.Teams = make([]string, len(teamNames))
	copy(market.Teams, teamNames)
	
	// Parse and validate payoff
	if market.Payoff == "" {
		return fmt.Errorf("market %s has no payoff defined", market.Name)
	}
	
	parsedPayoff, err := parsePayoff(market.Payoff)
	if err != nil {
		return fmt.Errorf("error parsing payoff for market %s: %v", market.Name, err)
	}
	market.ParsedPayoff = parsedPayoff
	
	// Validate payoff length matches all teams count
	if len(market.ParsedPayoff) != len(teamNames) {
		return fmt.Errorf("%s standard market payoff length (%d) does not match total teams count (%d)", 
			market.Name, len(market.ParsedPayoff), len(teamNames))
	}
	
	return nil
}

// validateAndInitializeMarkets validates markets against current teams and initializes them
func validateAndInitializeMarkets(markets []Market, currentTeams map[string][]string, eventsByLeague map[string][]MatchResult, latestSeason string) error {
	for i := range markets {
		market := &markets[i]
		
		// Validate league field
		if market.League == "" {
			return fmt.Errorf("market %s has no league specified", market.Name)
		}
		
		// Check if league is valid
		teamNamesForLeague, exists := currentTeams[market.League]
		if !exists {
			return fmt.Errorf("market %s references unknown league %s", market.Name, market.League)
		}
		
		// Validate that market doesn't have both include and exclude
		if len(market.Include) > 0 && len(market.Exclude) > 0 {
			return fmt.Errorf("market %s cannot have both include and exclude fields", market.Name)
		}
		
		// Initialize teams based on include/exclude
		var err error
		if len(market.Include) > 0 {
			err = initIncludeMarket(teamNamesForLeague, market)
		} else if len(market.Exclude) > 0 {
			err = initExcludeMarket(teamNamesForLeague, market)
		} else {
			err = initStandardMarket(teamNamesForLeague, market)
		}
		
		if err != nil {
			return err
		}
	}
	
	return nil
}