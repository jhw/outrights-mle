package outrightsmle

// convertSeasonToYear converts season string to starting year
// e.g., "2425" -> 2024, "2324" -> 2023
func convertSeasonToYear(season string) int {
	// Convert season string to starting year
	if len(season) != 4 {
		return 2014 // Default to oldest season
	}
	
	// Parse first two digits and add 2000
	year := 0
	if season[0] >= '0' && season[0] <= '9' {
		year += int(season[0]-'0') * 10
	}
	if season[1] >= '0' && season[1] <= '9' {
		year += int(season[1] - '0')
	}
	
	return 2000 + year
}

// Additional parsing/formatting utilities can be added here:
// - parseTeamName() for handling alternate names
// - formatSeason() for standardizing season formats
// - parseMatchDate() for date parsing
// etc.