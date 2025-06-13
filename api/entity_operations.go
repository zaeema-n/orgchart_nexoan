package api

import (
	"fmt"
	"strings"
	"time"

	"orgchart_nexoan/models"
)

// CreateGovernmentNode creates the initial government node
func (c *Client) CreateGovernmentNode() (*models.Entity, error) {
	// Create the government entity
	governmentEntity := &models.Entity{
		ID:      "gov_01",
		Created: "2024-01-01T00:00:00Z",
		Kind: models.Kind{
			Major: "Organisation",
			Minor: "government",
		},
		Name: models.TimeBasedValue{
			StartTime: "2024-01-01T00:00:00Z",
			Value:     "Government of Sri Lanka",
		},
	}

	// Create the entity
	createdEntity, err := c.CreateEntity(governmentEntity)
	if err != nil {
		return nil, fmt.Errorf("failed to create government entity: %w", err)
	}

	return createdEntity, nil
}

// AddOrgEntity creates a new entity and establishes its relationship with a parent entity.
// Assumes the parent entity already exists.
func (c *Client) AddOrgEntity(transaction map[string]interface{}, entityCounters map[string]int) (int, error) {
	// Extract details from the transaction
	parent := transaction["parent"].(string)
	child := transaction["child"].(string)
	dateStr := transaction["date"].(string)
	parentType := transaction["parent_type"].(string)
	childType := transaction["child_type"].(string)
	relType := transaction["rel_type"].(string)
	transactionID := transaction["transaction_id"].(string)

	// Parse the date
	date, err := time.Parse("2006-01-02", strings.TrimSpace(dateStr))
	if err != nil {
		return 0, fmt.Errorf("failed to parse date: %w", err)
	}
	dateISO := date.Format(time.RFC3339)

	// Generate new entity ID
	if _, exists := entityCounters[childType]; !exists {
		return 0, fmt.Errorf("unknown child type: %s", childType)
	}

	prefix := fmt.Sprintf("%s_%s", transactionID[:7], strings.ToLower(childType[:3]))
	entityCounter := entityCounters[childType] + 1
	newEntityID := fmt.Sprintf("%s_%d", prefix, entityCounter)

	// Get the parent entity ID
	searchCriteria := &models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: parentType,
		},
		Name: parent,
	}

	searchResults, err := c.SearchEntities(searchCriteria)
	if err != nil {
		return 0, fmt.Errorf("failed to search for parent entity: %w", err)
	}

	if len(searchResults) == 0 {
		return 0, fmt.Errorf("parent entity not found: %s", parent)
	}

	parentID := searchResults[0].ID

	// Create the new child entity
	childEntity := &models.Entity{
		ID: newEntityID,
		Kind: models.Kind{
			Major: "Organisation",
			Minor: childType,
		},
		Created:    dateISO,
		Terminated: "",
		Name: models.TimeBasedValue{
			StartTime: dateISO,
			Value:     child,
		},
		Metadata:      []models.MetadataEntry{},
		Attributes:    []models.AttributeEntry{},
		Relationships: []models.RelationshipEntry{},
	}

	// Create the child entity
	createdChild, err := c.CreateEntity(childEntity)
	if err != nil {
		return 0, fmt.Errorf("failed to create child entity: %w", err)
	}

	// Update the parent entity to add the relationship to the child
	parentEntity := &models.Entity{
		ID:         parentID,
		Kind:       models.Kind{},
		Created:    "",
		Terminated: "",
		Name:       models.TimeBasedValue{},
		Metadata:   []models.MetadataEntry{},
		Attributes: []models.AttributeEntry{},
		Relationships: []models.RelationshipEntry{
			{
				Key: fmt.Sprintf("%s_%s", parentID, createdChild.ID),
				Value: models.Relationship{
					RelatedEntityID: createdChild.ID,
					StartTime:       dateISO,
					EndTime:         "",
					ID:              fmt.Sprintf("%s_%s", parentID, createdChild.ID),
					Name:            relType,
				},
			},
		},
	}

	_, err = c.UpdateEntity(parentID, parentEntity)
	if err != nil {
		return 0, fmt.Errorf("failed to update parent entity: %w", err)
	}

	return entityCounter, nil
}

// TerminateOrgEntity terminates a specific relationship between parent and child at a given date
func (c *Client) TerminateOrgEntity(transaction map[string]interface{}) error {
	// Extract details from the transaction
	parent := transaction["parent"].(string)
	child := transaction["child"].(string)
	dateStr := transaction["date"].(string)
	parentType := transaction["parent_type"].(string)
	childType := transaction["child_type"].(string)
	relType := transaction["rel_type"].(string)

	// Parse the date
	date, err := time.Parse("2006-01-02", strings.TrimSpace(dateStr))
	if err != nil {
		return fmt.Errorf("failed to parse date: %w", err)
	}
	dateISO := date.Format(time.RFC3339)

	// Get the parent entity ID
	searchCriteria := &models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: parentType,
		},
		Name: parent,
	}
	parentResults, err := c.SearchEntities(searchCriteria)
	if err != nil {
		return fmt.Errorf("failed to search for parent entity: %w", err)
	}
	if len(parentResults) == 0 {
		return fmt.Errorf("parent entity not found: %s", parent)
	}
	parentID := parentResults[0].ID

	// Get the child entity ID
	searchCriteria.Kind.Minor = childType
	searchCriteria.Name = child
	childResults, err := c.SearchEntities(searchCriteria)
	if err != nil {
		return fmt.Errorf("failed to search for child entity: %w", err)
	}
	if len(childResults) == 0 {
		return fmt.Errorf("child entity not found: %s", child)
	}
	childID := childResults[0].ID

	// If we're terminating a minister, check for active departments
	if childType == "minister" {
		// Get all relationships for the minister
		relations, err := c.GetAllRelatedEntities(childID)
		if err != nil {
			return fmt.Errorf("failed to get minister's relationships: %w", err)
		}

		// Check for active departments
		for _, rel := range relations {
			if rel.Name == "AS_DEPARTMENT" && rel.EndTime == "" {
				return fmt.Errorf("cannot terminate minister with active departments")
			}
		}
	}

	// Get the specific relationship that is still active (no end date) -> this should give us the relationship(s) active for dateISO
	relations, err := c.GetRelatedEntities(parentID, &models.Relationship{
		RelatedEntityID: childID,
		Name:            relType,
		StartTime:       dateISO,
	})
	if err != nil {
		return fmt.Errorf("failed to get relationship: %w", err)
	}

	// FIXME: Is it possible to have more than one active relationship? For orgchart case only it won't happen
	// Find the active relationship (no end time)
	var activeRel *models.Relationship
	for _, rel := range relations {
		if rel.RelatedEntityID == childID && rel.EndTime == "" {
			activeRel = &rel
			break
		}
	}

	if activeRel == nil {
		return fmt.Errorf("no active relationship found between %s and %s with type %s", parentID, childID, relType)
	}

	// Update the relationship to set the end date
	_, err = c.UpdateEntity(parentID, &models.Entity{
		ID: parentID,
		Relationships: []models.RelationshipEntry{
			{
				Key: activeRel.ID,
				Value: models.Relationship{
					EndTime: dateISO,
					ID:      activeRel.ID,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to terminate relationship: %w", err)
	}

	return nil
}

// MoveDepartment moves a department from one minister to another
func (c *Client) MoveDepartment(transaction map[string]interface{}) error {
	// Extract details from the transaction
	newParent := transaction["new_parent"].(string)
	oldParent := transaction["old_parent"].(string)
	child := transaction["child"].(string)
	dateStr := transaction["date"].(string)

	// Parse the date
	date, err := time.Parse("2006-01-02", strings.TrimSpace(dateStr))
	if err != nil {
		return fmt.Errorf("failed to parse date: %w", err)
	}
	dateISO := date.Format(time.RFC3339)

	// Get the new minister (parent) entity ID
	newParentResults, err := c.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "minister",
		},
		Name: newParent,
	})
	if err != nil {
		return fmt.Errorf("failed to search for new parent entity: %w", err)
	}
	if len(newParentResults) == 0 {
		return fmt.Errorf("new parent entity not found: %s", newParent)
	}
	newParentID := newParentResults[0].ID

	// Get the department (child) entity ID
	childResults, err := c.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "department",
		},
		Name: child,
	})
	if err != nil {
		return fmt.Errorf("failed to search for child entity: %w", err)
	}
	if len(childResults) == 0 {
		return fmt.Errorf("child entity not found: %s", child)
	}
	childID := childResults[0].ID

	// Create new relationship between new minister and department
	newRelationship := &models.Entity{
		ID: newParentID,
		Relationships: []models.RelationshipEntry{
			{
				Key: fmt.Sprintf("%s_%s", newParentID, childID),
				Value: models.Relationship{
					RelatedEntityID: childID,
					StartTime:       dateISO,
					EndTime:         "",
					ID:              fmt.Sprintf("%s_%s", newParentID, childID),
					Name:            "AS_DEPARTMENT",
				},
			},
		},
	}

	_, err = c.UpdateEntity(newParentID, newRelationship)
	if err != nil {
		return fmt.Errorf("failed to create new relationship: %w", err)
	}

	// Terminate the old relationship
	terminateTransaction := map[string]interface{}{
		"parent":      oldParent,
		"child":       child,
		"date":        dateStr,
		"parent_type": "minister",
		"child_type":  "department",
		"rel_type":    "AS_DEPARTMENT",
	}

	err = c.TerminateOrgEntity(terminateTransaction)
	if err != nil {
		return fmt.Errorf("failed to terminate old relationship: %w", err)
	}

	return nil
}

// RenameMinister renames a minister and transfers all its departments to the new minister
func (c *Client) RenameMinister(transaction map[string]interface{}, entityCounters map[string]int) (int, error) {
	// Extract details from the transaction
	oldName := transaction["old"].(string)
	newName := transaction["new"].(string)
	dateStr := transaction["date"].(string)
	relType := transaction["type"].(string)
	transactionID := transaction["transaction_id"]

	// Parse the date
	date, err := time.Parse("2006-01-02", strings.TrimSpace(dateStr))
	if err != nil {
		return 0, fmt.Errorf("failed to parse date: %w", err)
	}
	dateISO := date.Format(time.RFC3339)

	// Get the old minister's ID
	oldMinisterResults, err := c.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "minister",
		},
		Name: oldName,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to search for old minister: %w", err)
	}
	if len(oldMinisterResults) == 0 {
		return 0, fmt.Errorf("old minister not found: %s", oldName)
	}
	oldMinisterID := oldMinisterResults[0].ID

	// Create new minister
	addEntityTransaction := map[string]interface{}{
		"parent":         "Government of Sri Lanka",
		"child":          newName,
		"date":           dateStr,
		"parent_type":    "government",
		"child_type":     "minister",
		"rel_type":       relType,
		"transaction_id": transactionID,
	}

	// Create the new minister
	newMinisterCounter, err := c.AddOrgEntity(addEntityTransaction, entityCounters)
	if err != nil {
		return 0, fmt.Errorf("failed to create new minister: %w", err)
	}

	// Get the new minister's ID
	newMinisterResults, err := c.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "minister",
		},
		Name: newName,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to search for new minister: %w", err)
	}
	if len(newMinisterResults) == 0 {
		return 0, fmt.Errorf("new minister not found: %s", newName)
	}
	newMinisterID := newMinisterResults[0].ID

	// Get all active departments of the old minister
	oldRelations, err := c.GetAllRelatedEntities(oldMinisterID)
	if err != nil {
		return 0, fmt.Errorf("failed to get old minister's relationships: %w", err)
	}

	// Transfer each active department to the new minister
	for _, rel := range oldRelations {
		if rel.Name == "AS_DEPARTMENT" && rel.EndTime == "" {
			// Get the department name using its ID
			departmentResults, err := c.SearchEntities(&models.SearchCriteria{
				ID: rel.RelatedEntityID,
			})
			if err != nil {
				return 0, fmt.Errorf("failed to search for department: %w", err)
			}

			if len(departmentResults) == 0 {
				return 0, fmt.Errorf("failed to find department with ID: %s", rel.RelatedEntityID)
			}

			// Create new relationship between new minister and department
			newRelationship := &models.Entity{
				ID: newMinisterID,
				Relationships: []models.RelationshipEntry{
					{
						Key: fmt.Sprintf("%s_%s", newMinisterID, rel.RelatedEntityID),
						Value: models.Relationship{
							RelatedEntityID: rel.RelatedEntityID,
							StartTime:       dateISO,
							EndTime:         "",
							ID:              fmt.Sprintf("%s_%s", newMinisterID, rel.RelatedEntityID),
							Name:            "AS_DEPARTMENT",
						},
					},
				},
			}

			_, err = c.UpdateEntity(newMinisterID, newRelationship)
			if err != nil {
				return 0, fmt.Errorf("failed to create new department relationship: %w", err)
			}

			// Terminate the old relationship
			terminateTransaction := map[string]interface{}{
				"parent":      oldName,
				"child":       departmentResults[0].Name,
				"date":        dateStr,
				"parent_type": "minister",
				"child_type":  "department",
				"rel_type":    "AS_DEPARTMENT",
			}

			err = c.TerminateOrgEntity(terminateTransaction)
			if err != nil {
				return 0, fmt.Errorf("failed to terminate old department relationship: %w", err)
			}
		}
	}

	// Terminate the old minister's relationship with government
	terminateGovTransaction := map[string]interface{}{
		"parent":      "Government of Sri Lanka",
		"child":       oldName,
		"date":        dateStr,
		"parent_type": "government",
		"child_type":  "minister",
		"rel_type":    relType,
	}

	err = c.TerminateOrgEntity(terminateGovTransaction)
	if err != nil {
		return 0, fmt.Errorf("failed to terminate old minister's government relationship: %w", err)
	}

	// Create RENAMED_TO relationship
	renameRelationship := &models.Entity{
		ID: oldMinisterID,
		Relationships: []models.RelationshipEntry{
			{
				Key: fmt.Sprintf("%s_%s", oldMinisterID, newMinisterID),
				Value: models.Relationship{
					RelatedEntityID: newMinisterID,
					StartTime:       dateISO,
					EndTime:         "",
					ID:              fmt.Sprintf("%s_%s", oldMinisterID, newMinisterID),
					Name:            "RENAMED_TO",
				},
			},
		},
	}

	_, err = c.UpdateEntity(oldMinisterID, renameRelationship)
	if err != nil {
		return 0, fmt.Errorf("failed to create RENAMED_TO relationship: %w", err)
	}

	return newMinisterCounter, nil
}

// MergeMinisters merges multiple ministers into a new minister
func (c *Client) MergeMinisters(transaction map[string]interface{}, entityCounters map[string]int) (int, error) {
	// Extract details from the transaction
	oldMinistersStr := transaction["old"].(string)
	newMinister := transaction["new"].(string)
	dateStr := transaction["date"].(string)
	transactionID := transaction["transaction_id"].(string)

	// Parse the date
	date, err := time.Parse("2006-01-02", strings.TrimSpace(dateStr))
	if err != nil {
		return 0, fmt.Errorf("failed to parse date: %w", err)
	}
	dateISO := date.Format(time.RFC3339)

	// Parse old ministers list
	oldMinisters := strings.Split(strings.Trim(oldMinistersStr, "[]"), ",")
	for i := range oldMinisters {
		oldMinisters[i] = strings.TrimSpace(oldMinisters[i])
	}

	// 1. Create new minister using AddEntity
	addEntityTransaction := map[string]interface{}{
		"parent":         "Government of Sri Lanka",
		"child":          newMinister,
		"date":           dateStr,
		"parent_type":    "government",
		"child_type":     "minister",
		"rel_type":       "AS_MINISTER",
		"transaction_id": transactionID,
	}

	newMinisterCounter, err := c.AddOrgEntity(addEntityTransaction, entityCounters)
	if err != nil {
		return 0, fmt.Errorf("failed to create new minister: %w", err)
	}

	// Get the new minister's ID
	newMinisterResults, err := c.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "minister",
		},
		Name: newMinister,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to search for new minister: %w", err)
	}
	if len(newMinisterResults) == 0 {
		return 0, fmt.Errorf("new minister not found: %s", newMinister)
	}
	newMinisterID := newMinisterResults[0].ID

	// For each old minister
	for _, oldMinister := range oldMinisters {
		// Get the old minister's ID
		oldMinisterResults, err := c.SearchEntities(&models.SearchCriteria{
			Kind: &models.Kind{
				Major: "Organisation",
				Minor: "minister",
			},
			Name: oldMinister,
		})
		if err != nil {
			return 0, fmt.Errorf("failed to search for old minister: %w", err)
		}
		if len(oldMinisterResults) == 0 {
			return 0, fmt.Errorf("old minister not found: %s", oldMinister)
		}
		oldMinisterID := oldMinisterResults[0].ID

		// 2. Move old minister's departments to new minister
		oldRelations, err := c.GetAllRelatedEntities(oldMinisterID)
		if err != nil {
			return 0, fmt.Errorf("failed to get old minister's relationships: %w", err)
		}

		for _, rel := range oldRelations {
			if rel.Name == "AS_DEPARTMENT" && rel.EndTime == "" {
				// Get the department name using its ID
				departmentResults, err := c.SearchEntities(&models.SearchCriteria{
					ID: rel.RelatedEntityID,
				})
				if err != nil {
					return 0, fmt.Errorf("failed to search for department: %w", err)
				}
				if len(departmentResults) == 0 {
					return 0, fmt.Errorf("failed to find department with ID: %s", rel.RelatedEntityID)
				}

				// Move department to new minister
				moveTransaction := map[string]interface{}{
					"old_parent": oldMinister,
					"new_parent": newMinister,
					"child":      departmentResults[0].Name,
					"type":       "AS_DEPARTMENT",
					"date":       dateStr,
				}

				err = c.MoveDepartment(moveTransaction)
				if err != nil {
					return 0, fmt.Errorf("failed to move department: %w", err)
				}
			}
		}

		// 3. Terminate gov -> old minister relationship
		terminateGovTransaction := map[string]interface{}{
			"parent":      "Government of Sri Lanka",
			"child":       oldMinister,
			"date":        dateStr,
			"parent_type": "government",
			"child_type":  "minister",
			"rel_type":    "AS_MINISTER",
		}

		err = c.TerminateOrgEntity(terminateGovTransaction)
		if err != nil {
			return 0, fmt.Errorf("failed to terminate old minister's government relationship: %w", err)
		}

		// 4. Create old minister -> new minister MERGED_INTO relationship
		mergedIntoRelationship := &models.Entity{
			ID: oldMinisterID,
			Relationships: []models.RelationshipEntry{
				{
					Key: fmt.Sprintf("%s_%s", oldMinisterID, newMinisterID),
					Value: models.Relationship{
						RelatedEntityID: newMinisterID,
						StartTime:       dateISO,
						EndTime:         "",
						ID:              fmt.Sprintf("%s_%s", oldMinisterID, newMinisterID),
						Name:            "MERGED_INTO",
					},
				},
			},
		}

		_, err = c.UpdateEntity(oldMinisterID, mergedIntoRelationship)
		if err != nil {
			return 0, fmt.Errorf("failed to create MERGED_INTO relationship: %w", err)
		}
	}

	return newMinisterCounter, nil
}

// AddPersonEntity creates a new person entity and establishes its relationship with a parent entity.
// Assumes the parent entity already exists.
func (c *Client) AddPersonEntity(transaction map[string]interface{}, entityCounters map[string]int) (int, error) {
	// Extract details from the transaction
	parent := transaction["parent"].(string)
	child := transaction["child"].(string)
	dateStr := transaction["date"].(string)
	parentType := transaction["parent_type"].(string)
	childType := transaction["child_type"].(string)
	relType := transaction["rel_type"].(string)
	transactionID := transaction["transaction_id"].(string)

	// Parse the date
	date, err := time.Parse("2006-01-02", strings.TrimSpace(dateStr))
	if err != nil {
		return 0, fmt.Errorf("failed to parse date: %w", err)
	}
	dateISO := date.Format(time.RFC3339)

	// Get the parent entity ID
	searchCriteria := &models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: parentType,
		},
		Name: parent,
	}

	searchResults, err := c.SearchEntities(searchCriteria)
	if err != nil {
		return 0, fmt.Errorf("failed to search for parent entity: %w", err)
	}

	if len(searchResults) == 0 {
		return 0, fmt.Errorf("parent entity not found: %s", parent)
	}

	parentID := searchResults[0].ID

	// Check if person already exists (search across all person types)
	personSearchCriteria := &models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Person",
		},
		Name: child,
	}

	personResults, err := c.SearchEntities(personSearchCriteria)
	if err != nil {
		return 0, fmt.Errorf("failed to search for person entity: %w", err)
	}

	if len(personResults) > 1 {
		return 0, fmt.Errorf("multiple entities found for person: %s", child)
	}

	var childID string

	if len(personResults) == 1 {
		// Person exists, use existing ID
		childID = personResults[0].ID
	} else {
		// Generate new entity ID
		if _, exists := entityCounters[childType]; !exists {
			return 0, fmt.Errorf("unknown child type: %s", childType)
		}

		prefix := fmt.Sprintf("%s_%s", transactionID[:7], strings.ToLower(childType[:3]))
		entityCounters[childType] = entityCounters[childType] + 1
		newEntityID := fmt.Sprintf("%s_%d", prefix, entityCounters[childType])

		// Create the new child entity
		childEntity := &models.Entity{
			ID: newEntityID,
			Kind: models.Kind{
				Major: "Person",
				Minor: childType,
			},
			Created:    dateISO,
			Terminated: "",
			Name: models.TimeBasedValue{
				StartTime: dateISO,
				Value:     child,
			},
			Metadata:      []models.MetadataEntry{},
			Attributes:    []models.AttributeEntry{},
			Relationships: []models.RelationshipEntry{},
		}

		// Create the child entity
		createdChild, err := c.CreateEntity(childEntity)
		if err != nil {
			return 0, fmt.Errorf("failed to create child entity: %w", err)
		}
		childID = createdChild.ID
	}

	// Update the parent entity to add the relationship to the child
	parentEntity := &models.Entity{
		ID:         parentID,
		Kind:       models.Kind{},
		Created:    "",
		Terminated: "",
		Name:       models.TimeBasedValue{},
		Metadata:   []models.MetadataEntry{},
		Attributes: []models.AttributeEntry{},
		Relationships: []models.RelationshipEntry{
			{
				Key: fmt.Sprintf("%s_%s", parentID, childID),
				Value: models.Relationship{
					RelatedEntityID: childID,
					StartTime:       dateISO,
					EndTime:         "",
					ID:              fmt.Sprintf("%s_%s", parentID, childID),
					Name:            relType,
				},
			},
		},
	}

	_, err = c.UpdateEntity(parentID, parentEntity)
	if err != nil {
		return 0, fmt.Errorf("failed to update parent entity: %w", err)
	}

	return entityCounters[childType] + 1, nil
}

// TerminatePersonEntity terminates a specific relationship between Person type entity and another entity at a given date
func (c *Client) TerminatePersonEntity(transaction map[string]interface{}) error {
	// Extract details from the transaction
	parent := transaction["parent"].(string)
	child := transaction["child"].(string)
	dateStr := transaction["date"].(string)
	parentType := transaction["parent_type"].(string)
	childType := transaction["child_type"].(string)
	relType := transaction["rel_type"].(string)

	// Parse the date
	date, err := time.Parse("2006-01-02", strings.TrimSpace(dateStr))
	if err != nil {
		return fmt.Errorf("failed to parse date: %w", err)
	}
	dateISO := date.Format(time.RFC3339)

	// Get the parent entity ID
	searchCriteria := &models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: parentType,
		},
		Name: parent,
	}
	parentResults, err := c.SearchEntities(searchCriteria)
	if err != nil {
		return fmt.Errorf("failed to search for parent entity: %w", err)
	}
	if len(parentResults) == 0 {
		return fmt.Errorf("parent entity not found: %s", parent)
	}
	parentID := parentResults[0].ID

	// Get the child entity ID
	childSearchCriteria := &models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Person",
			Minor: childType,
		},
		Name: child,
	}

	childResults, err := c.SearchEntities(childSearchCriteria)
	if err != nil {
		return fmt.Errorf("failed to search for child entity: %w", err)
	}
	if len(childResults) == 0 {
		return fmt.Errorf("child entity not found: %s", child)
	}
	childID := childResults[0].ID

	// Get the specific relationship that is still active (no end date) -> this should give us the relationship(s) active for dateISO
	relations, err := c.GetRelatedEntities(parentID, &models.Relationship{
		RelatedEntityID: childID,
		Name:            relType,
		StartTime:       dateISO,
	})
	if err != nil {
		return fmt.Errorf("failed to get relationship: %w", err)
	}

	// FIXME: Is it possible to have more than one active relationship? For orgchart case only it won't happen
	// Find the active relationship (no end time)
	var activeRel *models.Relationship
	for _, rel := range relations {
		if rel.RelatedEntityID == childID && rel.EndTime == "" {
			activeRel = &rel
			break
		}
	}

	if activeRel == nil {
		return fmt.Errorf("no active relationship found between %s and %s with type %s", parentID, childID, relType)
	}

	// Update the relationship to set the end date
	_, err = c.UpdateEntity(parentID, &models.Entity{
		ID: parentID,
		Relationships: []models.RelationshipEntry{
			{
				Key: activeRel.ID,
				Value: models.Relationship{
					EndTime: dateISO,
					ID:      activeRel.ID,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to terminate relationship: %w", err)
	}

	return nil
}

// MovePerson moves a person from one portfolio to another (limits functionality to only minister)
// TODO: Take the parent type from the transaction such that this function can be used generic
//
//	for moving person from any institution to another
func (c *Client) MovePerson(transaction map[string]interface{}) error {
	// Extract details from the transaction
	newParent := transaction["new_parent"].(string)
	oldParent := transaction["old_parent"].(string)
	child := transaction["child"].(string)
	dateStr := transaction["date"].(string)
	relType := transaction["type"].(string)

	// Parse the date
	date, err := time.Parse("2006-01-02", strings.TrimSpace(dateStr))
	if err != nil {
		return fmt.Errorf("failed to parse date: %w", err)
	}
	dateISO := date.Format(time.RFC3339)

	// Get the new minister (parent) entity ID
	newParentResults, err := c.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "minister",
		},
		Name: newParent,
	})
	if err != nil {
		return fmt.Errorf("failed to search for new parent entity: %w", err)
	}
	if len(newParentResults) == 0 {
		return fmt.Errorf("new parent entity not found: %s", newParent)
	}
	newParentID := newParentResults[0].ID

	// Get the department (child) entity ID
	childResults, err := c.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Person",
			Minor: "citizen",
		},
		Name: child,
	})
	if err != nil {
		return fmt.Errorf("failed to search for child entity: %w", err)
	}
	if len(childResults) == 0 {
		return fmt.Errorf("child entity not found: %s", child)
	}
	childID := childResults[0].ID

	// Create new relationship between new minister and person
	newRelationship := &models.Entity{
		ID: newParentID,
		Relationships: []models.RelationshipEntry{
			{
				Key: fmt.Sprintf("%s_%s", newParentID, childID),
				Value: models.Relationship{
					RelatedEntityID: childID,
					StartTime:       dateISO,
					EndTime:         "",
					ID:              fmt.Sprintf("%s_%s", newParentID, childID),
					Name:            relType,
				},
			},
		},
	}

	_, err = c.UpdateEntity(newParentID, newRelationship)
	if err != nil {
		return fmt.Errorf("failed to create new relationship: %w", err)
	}

	// Terminate the old relationship
	terminateTransaction := map[string]interface{}{
		"parent":      oldParent,
		"child":       child,
		"date":        dateStr,
		"parent_type": "minister",
		"child_type":  "citizen",
		"rel_type":    relType,
	}

	err = c.TerminatePersonEntity(terminateTransaction)
	if err != nil {
		return fmt.Errorf("failed to terminate old relationship: %w", err)
	}

	return nil
}
