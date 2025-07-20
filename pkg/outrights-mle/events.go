package outrightsmle

import (
	"encoding/json"
	"fmt"
	"os"
)

// TeamConfig represents a team configuration from core-data
type TeamConfig struct {
	Name     string   `json:"name"`
	AltNames []string `json:"altNames,omitempty"`
}

// EventProcessor handles event data processing and analysis
type EventProcessor struct {
	events       []MatchResult
	debug        bool
	leagueGroups map[string][]string
}

// NewEventProcessor creates a new event processor
func NewEventProcessor(events []MatchResult, debug bool) *EventProcessor {
	return &EventProcessor{
		events: events,
		debug:  debug,
	}
}

// LoadLeagueGroups loads team configurations from core-data/teams files
func (ep *EventProcessor) LoadLeagueGroups() error {
	leagues := []string{"ENG1", "ENG2", "ENG3", "ENG4"}
	leagueGroups := make(map[string][]string)
	
	for _, league := range leagues {
		filename := fmt.Sprintf("core-data/%s-teams.json", league)
		
		// Check if file exists
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			// File doesn't exist, skip this league
			continue
		}
		
		file, err := os.Open(filename)
		if err != nil {
			return fmt.Errorf("opening teams file %s: %w", filename, err)
		}
		defer file.Close()
		
		var teams []TeamConfig
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&teams); err != nil {
			return fmt.Errorf("decoding teams JSON from %s: %w", filename, err)
		}
		
		// Extract team names
		var teamNames []string
		for _, team := range teams {
			teamNames = append(teamNames, team.Name)
		}
		
		leagueGroups[league] = teamNames
	}
	
	ep.leagueGroups = leagueGroups
	
	if ep.debug && len(leagueGroups) > 0 {
		fmt.Printf("📂 Loaded league groups: ")
		for league, teams := range leagueGroups {
			fmt.Printf("%s(%d teams) ", league, len(teams))
		}
		fmt.Printf("\n")
	}
	
	return nil
}

// GetLeagueGroups returns the loaded league groups
func (ep *EventProcessor) GetLeagueGroups() map[string][]string {
	return ep.leagueGroups
}

// FindLatestSeason finds the most recent season in the dataset
func (ep *EventProcessor) FindLatestSeason() string {
	latestSeason := ""
	for _, event := range ep.events {
		if event.Season > latestSeason {
			latestSeason = event.Season
		}
	}
	
	if ep.debug {
		fmt.Printf("🔍 Latest season detected: %s\n", latestSeason)
	}
	
	return latestSeason
}

// GroupEventsByLeague groups events by league code
func (ep *EventProcessor) GroupEventsByLeague() map[string][]MatchResult {
	eventsByLeague := make(map[string][]MatchResult)
	
	for _, event := range ep.events {
		eventsByLeague[event.League] = append(eventsByLeague[event.League], event)
	}
	
	if ep.debug {
		leagues := make([]string, 0, len(eventsByLeague))
		for league := range eventsByLeague {
			leagues = append(leagues, league)
		}
		fmt.Printf("🔍 Found events for leagues: %v\n", leagues)
	}
	
	return eventsByLeague
}

// DetectPromotedTeams finds teams that have changed leagues across seasons
func (ep *EventProcessor) DetectPromotedTeams() map[string]bool {
	promotedTeams := make(map[string]bool)
	
	if ep.debug {
		fmt.Printf("🔄 Detecting teams with league changes across 10 seasons...\n")
	}
	
	// Group teams by season and league to detect changes
	teamSeasonLeague := make(map[string]map[string]string) // team -> season -> league
	
	for _, event := range ep.events {
		if teamSeasonLeague[event.HomeTeam] == nil {
			teamSeasonLeague[event.HomeTeam] = make(map[string]string)
		}
		if teamSeasonLeague[event.AwayTeam] == nil {
			teamSeasonLeague[event.AwayTeam] = make(map[string]string)
		}
		
		teamSeasonLeague[event.HomeTeam][event.Season] = event.League
		teamSeasonLeague[event.AwayTeam][event.Season] = event.League
	}
	
	// Detect league changes for each team
	for team, seasonLeagues := range teamSeasonLeague {
		var seasons []string
		for season := range seasonLeagues {
			seasons = append(seasons, season)
		}
		
		// Sort seasons to check chronologically
		for i := 0; i < len(seasons)-1; i++ {
			for j := i + 1; j < len(seasons); j++ {
				if seasons[i] > seasons[j] {
					seasons[i], seasons[j] = seasons[j], seasons[i]
				}
			}
		}
		
		// Check for league changes between consecutive seasons
		var changes []string
		for i := 0; i < len(seasons)-1; i++ {
			currentLeague := seasonLeagues[seasons[i]]
			nextLeague := seasonLeagues[seasons[i+1]]
			
			if currentLeague != nextLeague {
				promotedTeams[team] = true
				// Track the change for debug output
				if currentLeague < nextLeague {
					changes = append(changes, fmt.Sprintf("📉 %s→%s", seasons[i], seasons[i+1]))
				} else {
					changes = append(changes, fmt.Sprintf("📈 %s→%s", seasons[i], seasons[i+1]))
				}
			}
		}
		
		// Debug output for teams with changes
		if ep.debug && len(changes) > 0 {
			fmt.Printf("  🔄 %s: %s\n", team, fmt.Sprintf("%s", changes[0]))
			for i := 1; i < len(changes); i++ {
				fmt.Printf("               %s\n", changes[i])
			}
		}
	}
	
	if ep.debug {
		fmt.Printf("📊 Found %d teams with historical league changes\n", len(promotedTeams))
	}
	
	return promotedTeams
}

// GetTeamsInSeason returns teams that played in a specific season for given events
func GetTeamsInSeason(events []MatchResult, season string) map[string]bool {
	teams := make(map[string]bool)
	for _, event := range events {
		if event.Season == season {
			teams[event.HomeTeam] = true
			teams[event.AwayTeam] = true
		}
	}
	return teams
}

// ExtractTeams gets unique team names from match data
func ExtractTeams(matches []MatchResult) []string {
	teamSet := make(map[string]bool)
	for _, match := range matches {
		teamSet[match.HomeTeam] = true
		teamSet[match.AwayTeam] = true
	}

	teams := make([]string, 0, len(teamSet))
	for team := range teamSet {
		teams = append(teams, team)
	}

	return teams
}

// ExtractLeagues gets unique league codes from match data
func ExtractLeagues(matches []MatchResult) []string {
	leagueSet := make(map[string]bool)
	for _, match := range matches {
		leagueSet[match.League] = true
	}

	leagues := make([]string, 0, len(leagueSet))
	for league := range leagueSet {
		leagues = append(leagues, league)
	}

	return leagues
}

// ExtractSeasons gets unique season codes from match data
func ExtractSeasons(matches []MatchResult) []string {
	seasonSet := make(map[string]bool)
	for _, match := range matches {
		seasonSet[match.Season] = true
	}

	seasons := make([]string, 0, len(seasonSet))
	for season := range seasonSet {
		seasons = append(seasons, season)
	}

	return seasons
}

// GlobalEntitySummary contains all unique entities found in match data
type GlobalEntitySummary struct {
	Teams   []string `json:"teams"`
	Leagues []string `json:"leagues"`
	Seasons []string `json:"seasons"`
}

// ExtractGlobalEntities extracts all unique teams, leagues, and seasons from match data
func ExtractGlobalEntities(matches []MatchResult) GlobalEntitySummary {
	return GlobalEntitySummary{
		Teams:   ExtractTeams(matches),
		Leagues: ExtractLeagues(matches),
		Seasons: ExtractSeasons(matches),
	}
}