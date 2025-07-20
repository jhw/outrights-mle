package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	outrightsmle "github.com/jhw/go-outrights-mle/pkg/outrights-mle"
)

// LeagueConfig holds configuration for each league
type LeagueConfig struct {
	Code           string // ENG1, ENG2, ENG3, ENG4
	FootballDataID string // E0, E1, E2, E3
	StartYear      int    // 2015 (for 2015-16 season)
	EndYear        int    // 2024 (for 2024-25 season)
}

// English leagues configuration - 10 years of data (2015-16 to 2024-25)
var englandLeagues = []LeagueConfig{
	{Code: "ENG1", FootballDataID: "E0", StartYear: 2015, EndYear: 2024},
	{Code: "ENG2", FootballDataID: "E1", StartYear: 2015, EndYear: 2024},
	{Code: "ENG3", FootballDataID: "E2", StartYear: 2015, EndYear: 2024},
	{Code: "ENG4", FootballDataID: "E3", StartYear: 2015, EndYear: 2024},
}

// FetchAllEvents downloads all football events from football-data.co.uk
// Returns a single concatenated list of all matches across all leagues and seasons
func FetchAllEvents() ([]outrightsmle.MatchResult, error) {
	var allEvents []outrightsmle.MatchResult

	fmt.Printf("📥 Fetching football events from football-data.co.uk...\n")
	fmt.Printf("    Leagues: ENG1-4, Seasons: 2015-16 to 2024-25\n")
	fmt.Printf("    Rate limiting: 1s between requests + exponential backoff\n\n")

	client := &http.Client{Timeout: 30 * time.Second}
	
	totalRequests := 0
	for _, league := range englandLeagues {
		totalRequests += (league.EndYear - league.StartYear + 1)
	}

	requestCount := 0
	startTime := time.Now()

	for _, league := range englandLeagues {
		fmt.Printf("🏈 Processing %s (%s)...\n", league.Code, league.FootballDataID)

		for year := league.StartYear; year <= league.EndYear; year++ {
			requestCount++
			season := fmt.Sprintf("%02d%02d", year%100, (year+1)%100) // "1516", "1617", etc.
			
			fmt.Printf("  📅 Season %d-%02d (%s) [%d/%d]", year, (year+1)%100, season, requestCount, totalRequests)

			events, err := fetchSeasonEvents(client, league, season)
			if err != nil {
				fmt.Printf(" ❌ Error: %v\n", err)
				continue
			}

			allEvents = append(allEvents, events...)
			fmt.Printf(" ✓ %d events\n", len(events))
		}
		fmt.Printf("  ✓ %s complete\n\n", league.Code)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("🎯 Data fetching complete!\n")
	fmt.Printf("   Total events: %d\n", len(allEvents))
	fmt.Printf("   Total time: %v\n", elapsed)
	fmt.Printf("   Average per request: %v\n", elapsed/time.Duration(requestCount))

	return allEvents, nil
}

// fetchSeasonEvents downloads and parses events for a single league season
func fetchSeasonEvents(client *http.Client, league LeagueConfig, season string) ([]outrightsmle.MatchResult, error) {
	url := fmt.Sprintf("https://www.football-data.co.uk/mmz4281/%s/%s.csv", season, league.FootballDataID)

	// Rate limiting and retry logic
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 2s, 4s, 8s
			backoffDelay := time.Duration(2<<uint(attempt-1)) * time.Second
			time.Sleep(backoffDelay)
		} else {
			// Be a good net citizen - wait 1 second between requests
			time.Sleep(1 * time.Second)
		}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		// Set browser-like user agent and friendly headers
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "text/csv,text/plain,*/*")

		resp, err := client.Do(req)
		if err != nil {
			if attempt < maxRetries-1 {
				continue // Retry on network error
			}
			return nil, fmt.Errorf("HTTP request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return parseCSVEvents(resp.Body, league.Code, season)
		}

		// Handle server busy errors with retry
		if resp.StatusCode == 503 && attempt < maxRetries-1 {
			continue
		}

		// Other HTTP errors
		if attempt == maxRetries-1 {
			return nil, fmt.Errorf("HTTP %d after %d attempts: %s", resp.StatusCode, maxRetries, url)
		}
	}

	return nil, fmt.Errorf("unexpected end of retry loop")
}

// parseCSVEvents parses the football-data.co.uk CSV format into MatchResult events
func parseCSVEvents(reader io.Reader, leagueCode, season string) ([]outrightsmle.MatchResult, error) {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1 // Allow variable field count

	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading CSV: %w", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("empty CSV file")
	}

	// Find column indices from header row
	header := records[0]
	dateCol := findColumn(header, "Date")
	homeTeamCol := findColumn(header, "HomeTeam")
	awayTeamCol := findColumn(header, "AwayTeam")
	homeGoalsCol := findColumn(header, "FTHG") // Full Time Home Goals
	awayGoalsCol := findColumn(header, "FTAG") // Full Time Away Goals

	if dateCol == -1 || homeTeamCol == -1 || awayTeamCol == -1 || homeGoalsCol == -1 || awayGoalsCol == -1 {
		return nil, fmt.Errorf("required columns not found in CSV header")
	}

	var events []outrightsmle.MatchResult

	// Parse data rows
	for _, record := range records[1:] {
		if len(record) <= max(dateCol, homeTeamCol, awayTeamCol, homeGoalsCol, awayGoalsCol) {
			continue // Skip malformed rows
		}

		// Parse date
		dateStr := strings.TrimSpace(record[dateCol])
		if dateStr == "" {
			continue
		}

		date, err := parseDate(dateStr)
		if err != nil {
			continue // Skip rows with invalid dates
		}

		// Parse team names
		homeTeam := strings.TrimSpace(record[homeTeamCol])
		awayTeam := strings.TrimSpace(record[awayTeamCol])
		if homeTeam == "" || awayTeam == "" {
			continue
		}

		// Parse goals
		homeGoals, err := strconv.Atoi(strings.TrimSpace(record[homeGoalsCol]))
		if err != nil {
			continue
		}

		awayGoals, err := strconv.Atoi(strings.TrimSpace(record[awayGoalsCol]))
		if err != nil {
			continue
		}

		// Create match event with 4-digit season format (e.g., "2425" for 2024-25)
		event := outrightsmle.MatchResult{
			Date:      date.Format("2006-01-02"),
			Season:    season, // Already in YYMM format (e.g., "2425")
			League:    leagueCode,
			HomeTeam:  homeTeam,
			AwayTeam:  awayTeam,
			HomeGoals: homeGoals,
			AwayGoals: awayGoals,
		}

		events = append(events, event)
	}

	if len(events) == 0 {
		return nil, fmt.Errorf("no valid events parsed from CSV")
	}

	return events, nil
}

// parseDate handles multiple date formats used by football-data.co.uk
func parseDate(dateStr string) (time.Time, error) {
	// Try different date formats
	formats := []string{
		"02/01/06",   // DD/MM/YY
		"2/1/06",     // D/M/YY
		"02/01/2006", // DD/MM/YYYY
		"2/1/2006",   // D/M/YYYY
	}

	for _, format := range formats {
		if date, err := time.Parse(format, dateStr); err == nil {
			return date, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

// findColumn finds the index of a column in the CSV header
func findColumn(header []string, columnName string) int {
	for i, col := range header {
		if strings.EqualFold(strings.TrimSpace(col), columnName) {
			return i
		}
	}
	return -1
}

// Helper function to find the maximum of multiple integers
func max(vals ...int) int {
	if len(vals) == 0 {
		return 0
	}
	maxVal := vals[0]
	for _, v := range vals[1:] {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}