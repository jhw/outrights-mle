package outrightsmle

import (
	"fmt"
	"strings"
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error in %s: %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

func (e ValidationErrors) Error() string {
	if len(e.Errors) == 0 {
		return "no validation errors"
	}
	
	var messages []string
	for _, err := range e.Errors {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}

// ValidateLeagueGroups validates that league groups configuration is valid against actual event data
func ValidateLeagueGroups(leagueGroups map[string][]string, globalEntities GlobalEntitySummary) error {
	var errors []ValidationError
	
	if leagueGroups == nil {
		// No validation needed if no league groups specified
		return nil
	}
	
	// Create lookup maps for efficient validation
	validLeagues := make(map[string]bool)
	for _, league := range globalEntities.Leagues {
		validLeagues[league] = true
	}
	
	validTeams := make(map[string]bool)
	for _, team := range globalEntities.Teams {
		validTeams[team] = true
	}
	
	// Validate each league in league groups
	for leagueKey, teams := range leagueGroups {
		// Validate league key exists in event data
		if !validLeagues[leagueKey] {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("leagueGroups[%s]", leagueKey),
				Message: fmt.Sprintf("league '%s' not found in event data (available: %v)", leagueKey, globalEntities.Leagues),
			})
			continue // Skip team validation if league is invalid
		}
		
		// Validate each team exists in event data
		for i, team := range teams {
			if !validTeams[team] {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("leagueGroups[%s][%d]", leagueKey, i),
					Message: fmt.Sprintf("team '%s' not found in event data for league '%s'", team, leagueKey),
				})
			}
		}
	}
	
	if len(errors) > 0 {
		return ValidationErrors{Errors: errors}
	}
	
	return nil
}

// ValidateMLERequest validates an MLE request for common issues
func ValidateMLERequest(request MLERequest, globalEntities GlobalEntitySummary) error {
	var errors []ValidationError
	
	// Validate league groups if specified
	if err := ValidateLeagueGroups(request.LeagueGroups, globalEntities); err != nil {
		if validationErrors, ok := err.(ValidationErrors); ok {
			errors = append(errors, validationErrors.Errors...)
		} else {
			errors = append(errors, ValidationError{
				Field:   "leagueGroups",
				Message: err.Error(),
			})
		}
	}
	
	// Add more validations here as needed...
	
	if len(errors) > 0 {
		return ValidationErrors{Errors: errors}
	}
	
	return nil
}