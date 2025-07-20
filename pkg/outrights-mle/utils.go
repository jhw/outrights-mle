package outrightsmle

import (
	"fmt"
	"strconv"
)

// convertSeasonToYear converts season string to starting year
// e.g., "2425" -> 2024, "2324" -> 2023
func convertSeasonToYear(season string) (int, error) {
	if len(season) != 4 {
		return 0, fmt.Errorf("season must be 4 digits, got %d characters: %q", len(season), season)
	}
	
	// Parse the first two digits as the starting year (e.g., "23" from "2324")
	yearSuffix, err := strconv.Atoi(season[:2])
	if err != nil {
		return 0, fmt.Errorf("invalid season format %q: first two characters must be digits", season)
	}
	
	return 2000 + yearSuffix, nil
}

// Additional parsing/formatting utilities can be added here:
// - parseTeamName() for handling alternate names
// - formatSeason() for standardizing season formats
// - parseMatchDate() for date parsing
// etc.