package api

import (
	"fmt"
	"strings"
	"time"

	"orgchart_nexoan/models"

	"github.com/google/uuid"
)

// GetMinisterByPresident retrieves a minister entity by president name and minister name
func (c *Client) GetMinisterByPresident(presidentName, ministerName string) (*models.Entity, error) {
	// Get the president entity using the helper function.
	presidentEntity, err := c.GetPresidentByGovernment(presidentName)
	if err != nil {
		return nil, err
	}
	presidentID := presidentEntity.ID

	// Get all minister relationships for the president
	ministerRelations, err := c.GetRelatedEntities(presidentID, &models.Relationship{
		Name:      "AS_MINISTER",
		Direction: "OUTGOING",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get president's relationships: %w", err)
	}

	// Find the minister with the specified name
	for _, rel := range ministerRelations {
		// Fetch the related entity (minister)
		ministerResults, err := c.SearchEntities(&models.SearchCriteria{
			ID: rel.RelatedEntityID,
		})
		if err != nil || len(ministerResults) == 0 {
			continue
		}
		minister := ministerResults[0]
		if isMinisterType(minister.Kind.Minor) && minister.Name == ministerName {
			// Convert SearchResult to Entity
			entity := &models.Entity{
				ID:         minister.ID,
				Kind:       minister.Kind,
				Created:    minister.Created,
				Terminated: minister.Terminated,
				Name: models.TimeBasedValue{
					Value: minister.Name,
				},
				Metadata:      []models.MetadataEntry{},
				Attributes:    []models.AttributeEntry{},
				Relationships: []models.RelationshipEntry{},
			}
			return entity, nil
		}
	}

	return nil, fmt.Errorf("minister '%s' not found under president '%s'", ministerName, presidentName)
}

// GetActiveMinisterByPresident retrieves an active minister entity by president name and minister name
// Returns an error if multiple active ministers with the same name are found
func (c *Client) GetActiveMinisterByPresident(presidentName, ministerName, dateISO string) (*models.Entity, error) {
	// Get the president entity using the helper function
	presidentEntity, err := c.GetPresidentByGovernment(presidentName, dateISO)
	if err != nil {
		return nil, err
	}
	presidentID := presidentEntity.ID

	// Get all minister relationships for the president
	presidentRelations, err := c.GetRelatedEntities(presidentID, &models.Relationship{
		Name: "AS_MINISTER",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get president's relationships: %w", err)
	}

	// Find active ministers with the specified name
	var activeMinisters []*models.Entity
	for _, rel := range presidentRelations {
		// Only consider active relationships (EndTime == "")
		if rel.EndTime != "" {
			continue
		}

		// Fetch the related entity (minister)
		ministerResults, err := c.SearchEntities(&models.SearchCriteria{
			ID: rel.RelatedEntityID,
		})
		if err != nil || len(ministerResults) == 0 {
			continue
		}
		minister := ministerResults[0]
		if isMinisterType(minister.Kind.Minor) && minister.Name == ministerName {
			// Convert SearchResult to Entity
			entity := &models.Entity{
				ID:         minister.ID,
				Kind:       minister.Kind,
				Created:    minister.Created,
				Terminated: minister.Terminated,
				Name: models.TimeBasedValue{
					Value: minister.Name,
				},
				Metadata:      []models.MetadataEntry{},
				Attributes:    []models.AttributeEntry{},
				Relationships: []models.RelationshipEntry{},
			}
			activeMinisters = append(activeMinisters, entity)
		}
	}

	// Check for multiple active ministers with the same name
	if len(activeMinisters) > 1 {
		return nil, fmt.Errorf("multiple active ministers found with name '%s' under president '%s'", ministerName, presidentName)
	}

	// Check if no active minister was found
	if len(activeMinisters) == 0 {
		return nil, fmt.Errorf("no active minister found with name '%s' under president '%s'", ministerName, presidentName)
	}

	return activeMinisters[0], nil
}

// RenameMinister renames a minister and transfers all its departments to the new minister
func (c *Client) RenameMinister(transaction map[string]interface{}, entityCounters map[string]int) (int, error) {
	// Extract details from the transaction
	oldName := transaction["old"].(string)
	newName := transaction["new"].(string)
	dateStr := transaction["date"].(string)
	relType := "AS_MINISTER"
	transactionID := transaction["transaction_id"]

	// Validate president name is provided
	presidentName, ok := transaction["president"].(string)
	if !ok || presidentName == "" {
		return 0, fmt.Errorf("president name is required and must be a non-empty string")
	}

	// Parse the date
	date, err := time.Parse("2006-01-02", strings.TrimSpace(dateStr))
	if err != nil {
		return 0, fmt.Errorf("failed to parse date: %w", err)
	}
	dateISO := date.Format(time.RFC3339)

	// Get the old minister's ID
	oldMinister, err := c.GetActiveMinisterByPresident(presidentName, oldName, dateISO)
	if err != nil {
		return 0, fmt.Errorf("failed to get old minister: %w", err)
	}
	oldMinisterID := oldMinister.ID

	// Create new minister
	addEntityTransaction := map[string]interface{}{
		"parent":         presidentName,
		"child":          newName,
		"date":           dateStr,
		"parent_type":    "president",
		"child_type":     ministerTypeFromName(newName),
		"rel_type":       relType,
		"transaction_id": transactionID,
		"president":      presidentName,
	}

	// Create the new minister
	newMinisterCounter, err := c.AddOrgEntity(addEntityTransaction, entityCounters)
	if err != nil {
		return 0, fmt.Errorf("failed to create new minister: %w", err)
	}

	// Get the new minister's ID
	newMinister, err := c.GetActiveMinisterByPresident(presidentName, newName, dateISO)
	if err != nil {
		return 0, fmt.Errorf("failed to get new minister: %w", err)
	}
	newMinisterID := newMinister.ID

	// Get all active departments of the old minister
	oldRelations, err := c.GetRelatedEntities(oldMinisterID, &models.Relationship{
		Name: "AS_DEPARTMENT",
	})
	if err != nil {
		return 0, fmt.Errorf("failed to get old minister's relationships: %w", err)
	}

	// Manually filter only active relationships (EndTime == "")
	var oldActiveRelations []models.Relationship
	for _, rel := range oldRelations {
		if rel.EndTime == "" {
			oldActiveRelations = append(oldActiveRelations, rel)
		}
	}

	// Transfer each active department to the new minister using MoveDepartment
	for _, rel := range oldActiveRelations {
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

		// Use MoveDepartment to move the department from old minister to new minister
		moveTransaction := map[string]interface{}{
			"old_parent":         oldName,
			"new_parent":         newName,
			"child":              departmentResults[0].Name,
			"date":               dateStr,
			"new_president_name": presidentName,
			"old_president_name": presidentName,
		}

		err = c.MoveDepartment(moveTransaction)
		if err != nil {
			return 0, fmt.Errorf("failed to move department: %w", err)
		}
	}

	// Move all active AS_ROLE assignments from old role nodes to new role nodes.
	oldMinisterNodeID, err := roleNodeID(oldMinisterID, "minister")
	if err != nil {
		return 0, fmt.Errorf("failed to resolve old minister role node id: %w", err)
	}
	oldSecretaryNodeID, err := roleNodeID(oldMinisterID, "secretary")
	if err != nil {
		return 0, fmt.Errorf("failed to resolve old secretary role node id: %w", err)
	}
	newMinisterNodeID, err := roleNodeID(newMinisterID, "minister")
	if err != nil {
		return 0, fmt.Errorf("failed to resolve new minister role node id: %w", err)
	}
	newSecretaryNodeID, err := roleNodeID(newMinisterID, "secretary")
	if err != nil {
		return 0, fmt.Errorf("failed to resolve new secretary role node id: %w", err)
	}

	if err := c.moveIncomingASRoles(oldMinisterNodeID, newMinisterNodeID, dateISO); err != nil {
		return 0, fmt.Errorf("failed to move minister role assignments during rename: %w", err)
	}
	if err := c.moveIncomingASRoles(oldSecretaryNodeID, newSecretaryNodeID, dateISO); err != nil {
		return 0, fmt.Errorf("failed to move secretary role assignments during rename: %w", err)
	}

	// Terminate the old minister's relationship with the president directly
	// We need to get the president ID first
	presidentEntity, err := c.GetPresidentByGovernment(presidentName, dateISO)
	if err != nil {
		return 0, fmt.Errorf("failed to get president entity: %w", err)
	}
	presidentID := presidentEntity.ID

	// Find the active relationship to terminate it
	presidentRelations, err := c.GetRelatedEntities(presidentID, &models.Relationship{
		Name:            "AS_MINISTER",
		RelatedEntityID: oldMinisterID,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to get relationship between president and minister: %w", err)
	}

	// Find the active relationship (EndTime == "")
	var activeRel *models.Relationship
	for _, rel := range presidentRelations {
		if rel.EndTime == "" {
			activeRel = &rel
			break
		}
	}

	if activeRel == nil {
		return 0, fmt.Errorf("no active relationship found between president and minister")
	}

	// Terminate the relationship directly
	terminateRelationship := &models.Entity{
		ID: presidentID,
		Relationships: []models.RelationshipEntry{
			{
				Key: activeRel.ID,
				Value: models.Relationship{
					EndTime: dateISO,
					ID:      activeRel.ID,
				},
			},
		},
	}

	_, err = c.UpdateEntity(presidentID, terminateRelationship)
	if err != nil {
		return 0, fmt.Errorf("failed to terminate old minister's government relationship: %w", err)
	}

	if err := c.terminateOrganisationStructureRelationship(oldMinisterID, dateISO); err != nil {
		return 0, fmt.Errorf("failed to terminate old minister AS_ORGANISATION: %w", err)
	}

	// Create RENAMED_TO relationship
	// Use transaction ID and current timestamp to ensure unique relationship ID
	currentTimestamp := fmt.Sprintf("%s_%s", strings.ReplaceAll(time.Now().Format(time.RFC3339), ":", "-"), uuid.New().String()[:8])
	uniqueRelationshipID := fmt.Sprintf("%s_%s_%s", oldMinisterID, newMinisterID, currentTimestamp)

	renameRelationship := &models.Entity{
		ID: oldMinisterID,
		Relationships: []models.RelationshipEntry{
			{
				Key: uniqueRelationshipID,
				Value: models.Relationship{
					RelatedEntityID: newMinisterID,
					StartTime:       dateISO,
					EndTime:         "",
					ID:              uniqueRelationshipID,
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

	// Validate president name is provided
	presidentName, ok := transaction["president"].(string)
	if !ok || presidentName == "" {
		return 0, fmt.Errorf("president name is required and must be a non-empty string")
	}

	// Parse the date
	date, err := time.Parse("2006-01-02", strings.TrimSpace(dateStr))
	if err != nil {
		return 0, fmt.Errorf("failed to parse date: %w", err)
	}
	dateISO := date.Format(time.RFC3339)

	// Parse old ministers list - use semicolons as separators to avoid comma conflicts
	trimmedStr := strings.Trim(oldMinistersStr, "[]")
	oldMinisters := strings.Split(trimmedStr, ";")
	for i := range oldMinisters {
		oldMinisters[i] = strings.TrimSpace(oldMinisters[i])
	}

	// 1. Create new minister using AddEntity
	addEntityTransaction := map[string]interface{}{
		"parent":         presidentName,
		"child":          newMinister,
		"date":           dateStr,
		"parent_type":    "president",
		"child_type":     ministerTypeFromName(newMinister),
		"rel_type":       "AS_MINISTER",
		"transaction_id": transactionID,
		"president":      presidentName,
	}

	newMinisterCounter, err := c.AddOrgEntity(addEntityTransaction, entityCounters)
	if err != nil {
		return 0, fmt.Errorf("failed to create new minister: %w", err)
	}

	// Get the new minister's ID
	newMinisterEntity, err := c.GetActiveMinisterByPresident(presidentName, newMinister, dateISO)
	if err != nil {
		return 0, fmt.Errorf("failed to get new minister: %w", err)
	}
	newMinisterID := newMinisterEntity.ID

	// For each old minister
	for _, oldMinister := range oldMinisters {
		// Get the old minister's ID
		oldMinisterEntity, err := c.GetActiveMinisterByPresident(presidentName, oldMinister, dateISO)
		if err != nil {
			return 0, fmt.Errorf("failed to get old minister: %w", err)
		}
		oldMinisterID := oldMinisterEntity.ID

		// 1. Move old minister's departments to new minister
		oldRelations, err := c.GetRelatedEntities(oldMinisterID, &models.Relationship{
			Name: "AS_DEPARTMENT",
		})
		if err != nil {
			return 0, fmt.Errorf("failed to get old minister's relationships: %w", err)
		}

		// Manually filter only active relationships (EndTime == "")
		var oldActiveRelations []models.Relationship
		for _, rel := range oldRelations {
			if rel.EndTime == "" {
				oldActiveRelations = append(oldActiveRelations, rel)
			}
		}

		for _, rel := range oldActiveRelations {
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
				"old_parent":         oldMinister,
				"new_parent":         newMinister,
				"child":              departmentResults[0].Name,
				"date":               dateStr,
				"new_president_name": presidentName,
				"old_president_name": presidentName,
			}

			err = c.MoveDepartment(moveTransaction)
			if err != nil {
				return 0, fmt.Errorf("failed to move department: %w", err)
			}
		}

		// 2. Terminate all active AS_ROLE assignments under old minister role nodes.
		oldMinisterNodeID, err := roleNodeID(oldMinisterID, "minister")
		if err != nil {
			return 0, fmt.Errorf("failed to resolve old minister role node id during merge: %w", err)
		}
		oldSecretaryNodeID, err := roleNodeID(oldMinisterID, "secretary")
		if err != nil {
			return 0, fmt.Errorf("failed to resolve old secretary role node id during merge: %w", err)
		}
		if err := c.terminateIncomingASRoles(oldMinisterNodeID, dateISO); err != nil {
			return 0, fmt.Errorf("failed to terminate minister role assignments during merge: %w", err)
		}
		if err := c.terminateIncomingASRoles(oldSecretaryNodeID, dateISO); err != nil {
			return 0, fmt.Errorf("failed to terminate secretary role assignments during merge: %w", err)
		}

		// 3. Terminate gov -> old minister relationship
		terminateGovTransaction := map[string]interface{}{
			"parent":      presidentName,
			"child":       oldMinister,
			"date":        dateStr,
			"parent_type": "citizen",
			"child_type":  ministerTypeFromName(oldMinister),
			"rel_type":    "AS_MINISTER",
		}

		err = c.TerminateOrgEntity(terminateGovTransaction)
		if err != nil {
			return 0, fmt.Errorf("failed to terminate old minister's government relationship: %w", err)
		}

		// 4. Create old minister -> new minister MERGED_INTO relationship
		// Use transaction ID and current timestamp to ensure unique relationship ID
		currentTimestamp := fmt.Sprintf("%s_%s", strings.ReplaceAll(time.Now().Format(time.RFC3339), ":", "-"), uuid.New().String()[:8])
		uniqueRelationshipID := fmt.Sprintf("%s_%s_%s", oldMinisterID, newMinisterID, currentTimestamp)

		mergedIntoRelationship := &models.Entity{
			ID: oldMinisterID,
			Relationships: []models.RelationshipEntry{
				{
					Key: uniqueRelationshipID,
					Value: models.Relationship{
						RelatedEntityID: newMinisterID,
						StartTime:       dateISO,
						EndTime:         "",
						ID:              uniqueRelationshipID,
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

// MoveMinister moves a minister from one president to another
func (c *Client) MoveMinister(transaction map[string]interface{}) error {
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

	// --- Get the new president (parent) entity ID ---
	newPresidentEntity, err := c.GetPresidentByGovernment(newParent, dateISO)
	if err != nil {
		return fmt.Errorf("failed to get new president entity: %w", err)
	}
	newParentID := newPresidentEntity.ID

	// --- Get the old president (parent) entity ID ---
	oldPresidentEntity, err := c.GetPresidentByGovernment(oldParent, dateISO)
	if err != nil {
		return fmt.Errorf("failed to get old president entity: %w", err)
	}
	oldParentID := oldPresidentEntity.ID

	// Get the minister (child) entity ID connected to the old president
	ministerEntity, err := c.GetActiveMinisterByPresident(oldParent, child, dateISO)
	if err != nil {
		return fmt.Errorf("minister entity '%s' not found or not active under old president '%s' on date %s: %w", child, oldParent, dateStr, err)
	}
	childID := ministerEntity.ID

	// Create new relationship between new president and minister
	// Use transaction ID and current timestamp to ensure unique relationship ID
	currentTimestamp := fmt.Sprintf("%s_%s", strings.ReplaceAll(time.Now().Format(time.RFC3339), ":", "-"), uuid.New().String()[:8])
	uniqueRelationshipID := fmt.Sprintf("%s_%s_%s", newParentID, childID, currentTimestamp)

	newRelationship := &models.Entity{
		ID: newParentID,
		Relationships: []models.RelationshipEntry{
			{
				Key: uniqueRelationshipID,
				Value: models.Relationship{
					RelatedEntityID: childID,
					StartTime:       dateISO,
					EndTime:         "",
					ID:              uniqueRelationshipID,
					Name:            "AS_MINISTER",
				},
			},
		},
	}

	_, err = c.UpdateEntity(newParentID, newRelationship)
	if err != nil {
		return fmt.Errorf("failed to create new relationship: %w", err)
	}

	// Find the active relationship to terminate it.
	oldPresidentRelations, err := c.GetRelatedEntities(oldParentID, &models.Relationship{
		Name:            "AS_MINISTER",
		RelatedEntityID: childID,
	})
	if err != nil {
		return fmt.Errorf("failed to get relationship between old president and minister: %w", err)
	}

	// Manually filter for active relationships (EndTime == "")
	var activeRel *models.Relationship
	for _, rel := range oldPresidentRelations {
		if rel.EndTime == "" {
			activeRel = &rel
			break
		}
	}

	// Only terminate if there is an active relationship
	if activeRel != nil {
		// Terminate the old relationship directly without cascading to people
		_, err = c.UpdateEntity(oldParentID, &models.Entity{
			ID: oldParentID,
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
			return fmt.Errorf("failed to terminate old relationship: %w", err)
		}
	}

	return nil
}
