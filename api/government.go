package api

import (
	"fmt"

	"orgchart_nexoan/internal/utils"
	"orgchart_nexoan/models"
)

// CreateGovernmentNode creates the initial government node
func (c *Client) CreateGovernmentNode() (*models.Entity, error) {
	// Create the government entity
	governmentEntity := &models.Entity{
		ID:      "gov_01",
		Created: "1978-09-07T00:00:00Z",
		Kind: models.Kind{
			Major: "Organisation",
			Minor: "government",
		},
		Name: models.TimeBasedValue{
			StartTime: "1978-09-07T00:00:00Z",
			Value:     "Government of Sri Lanka",
		},
	}

	// Create the entity
	createdEntity, err := c.CreateEntity(governmentEntity)
	if err != nil {
		return nil, fmt.Errorf("failed to create government entity: %w", err)
	}
	if err := c.ensureGovernmentOrgStructure(createdEntity.ID, createdEntity.Created, ""); err != nil {
		return nil, fmt.Errorf("failed to create government org structure: %w", err)
	}

	return createdEntity, nil
}

func (c *Client) getGovernmentEntityIDByName(governmentName string) (string, error) {
	searchResults, err := c.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "government",
		},
		Name: governmentName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to search for government entity: %w", err)
	}
	searchResults = utils.FilterByExactName(searchResults, governmentName)
	if len(searchResults) == 0 {
		return "", fmt.Errorf("government entity not found: %s", governmentName)
	}
	if len(searchResults) > 1 {
		return "", fmt.Errorf("multiple government entities found with name '%s'", governmentName)
	}
	return searchResults[0].ID, nil
}

// GetPresidentByGovernment retrieves a president entity by name using government president-role assignment.
// Pass an optional dateISO to require the assignment to be active at that date.
func (c *Client) GetPresidentByGovernment(presidentName string, dateISO ...string) (*models.Entity, error) {
	activeAt := ""
	if len(dateISO) > 0 && dateISO[0] != "" {
		activeAt = dateISO[0]
	}

	// Get the president entity ID.
	presidentResults, err := c.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Person",
			Minor: "citizen",
		},
		Name: presidentName,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to search for president entity: %w", err)
	}

	// Filter for exact name match
	presidentResults = utils.FilterByExactName(presidentResults, presidentName)

	if len(presidentResults) == 0 {
		return nil, fmt.Errorf("president entity not found: %s", presidentName)
	}

	if len(presidentResults) > 1 {
		return nil, fmt.Errorf("multiple entities found with name '%s'", presidentName)
	}

	governmentID, err := c.getGovernmentEntityIDByName("Government of Sri Lanka")
	if err != nil {
		return nil, err
	}
	presidentNodeID, err := governmentRoleNodeID(governmentID, "president")
	if err != nil {
		return nil, err
	}

	// Find the president by checking if they have AS_ROLE relationship to the government's president role node.
	for _, president := range presidentResults {
		relCriteria := &models.Relationship{
			Name:            "AS_ROLE",
			RelatedEntityID: presidentNodeID,
			Direction:       "OUTGOING",
		}
		if activeAt != "" {
			relCriteria.ActiveAt = activeAt
		}
		// Check if this citizen has AS_ROLE relationship to the president role node.
		presidentRelations, err := c.GetRelatedEntities(president.ID, relCriteria)
		if err != nil || len(presidentRelations) == 0 {
			continue
		}
		// Convert SearchResult to Entity
		entity := &models.Entity{
			ID:         president.ID,
			Kind:       president.Kind,
			Created:    president.Created,
			Terminated: president.Terminated,
			Name: models.TimeBasedValue{
				Value: president.Name,
			},
			Metadata:      []models.MetadataEntry{},
			Attributes:    []models.AttributeEntry{},
			Relationships: []models.RelationshipEntry{},
		}
		return entity, nil
	}

	if activeAt != "" {
		return nil, fmt.Errorf("president entity not found or not active at %s: %s", activeAt, presidentName)
	}
	return nil, fmt.Errorf("president entity not found or not active: %s", presidentName)
}
