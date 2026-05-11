package api

import (
	"fmt"
	"strings"
	"time"

	"orgchart_nexoan/internal/utils"
	"orgchart_nexoan/models"

	"github.com/google/uuid"
)

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

	// Use effective child type for counter lookups
	effectiveChildType := childType
	if isMinisterType(childType) {
		effectiveChildType = "minister"
	}

	// Generate new entity ID
	if _, exists := entityCounters[effectiveChildType]; !exists {
		return 0, fmt.Errorf("unknown child type: %s", childType)
	}

	// Get the part before the first underscore for the prefix
	prefixPart := strings.Split(transactionID, "_")[0]
	prefix := fmt.Sprintf("%s_%s", prefixPart, strings.ToLower(childType[:3]))
	entityCounter := entityCounters[effectiveChildType] + 1
	newEntityID := fmt.Sprintf("%s_%d", prefix, entityCounter)

	// Get the parent entity ID based on the child type
	var parentID string

	if isMinisterType(childType) {
		// For ministers, parent should be a president (Person type) - presidents are citizens with AS_PRESIDENT relationship
		if parentType != "president" && parentType != "citizen" {
			return 0, fmt.Errorf("minister must be attached to a president, got parent_type: %s", parentType)
		}

		// Removed below: for now if a president creates the same minister again it will create a new entity
		// Check if minister already exists under this president
		// _, err := c.GetMinisterByPresident(parent, child, dateISO)
		// if err == nil {
		// 	// Minister already exists, return error
		// 	return 0, fmt.Errorf("minister '%s' already exists under president '%s'", child, parent)
		// }

		// Get the president entity active at transaction date.
		presidentEntity, err := c.GetPresidentByGovernment(parent, dateISO)
		if err != nil {
			return 0, fmt.Errorf("failed to get parent president entity: %w", err)
		}
		parentID = presidentEntity.ID

	} else if childType == "department" {
		// For departments, parent should be a minister, but we need to verify it's the correct minister
		if !isMinisterType(parentType) {
			return 0, fmt.Errorf("department must be attached to a minister, got parent_type: %s", parentType)
		}

		// Get president name from transaction
		presidentName, ok := transaction["president"].(string)
		if !ok || presidentName == "" {
			return 0, fmt.Errorf("president name is required and must be a non-empty string when adding a department")
		}

		// Check if a department with the same name already exists
		existingDepartmentResults, err := c.SearchEntities(&models.SearchCriteria{
			Kind: &models.Kind{
				Major: "Organisation",
				Minor: "department",
			},
			Name: child,
		})
		if err != nil {
			return 0, fmt.Errorf("failed to search for existing department: %w", err)
		}
		// Filter for exact name match before duplicate check
		existingDepartmentResults = utils.FilterByExactName(existingDepartmentResults, child)
		if len(existingDepartmentResults) > 0 {
			return 0, fmt.Errorf("department with name '%s' already exists", child)
		}

		// Use GetMinisterByPresident to ensure we get the correct minister under the correct president
		ministerEntity, err := c.GetActiveMinisterByPresident(presidentName, parent, dateISO)
		if err != nil {
			return 0, fmt.Errorf("failed to get parent minister entity: %w", err)
		}

		parentID = ministerEntity.ID

	} else {
		// For other entity types, use the original logic
		majorType := "Organisation"
		if parentType == "citizen" {
			majorType = "Person"
		}
		searchCriteria := &models.SearchCriteria{
			Kind: &models.Kind{
				Major: majorType,
				Minor: parentType,
			},
			Name: parent,
		}

		searchResults, err := c.SearchEntities(searchCriteria)
		if err != nil {
			return 0, fmt.Errorf("failed to search for parent entity: %w", err)
		}

		// Filter for exact name match
		searchResults = utils.FilterByExactName(searchResults, parent)

		if len(searchResults) == 0 {
			return 0, fmt.Errorf("parent entity not found: %s", parent)
		}
		if len(searchResults) > 1 {
			return 0, fmt.Errorf("multiple parent entities found with name '%s'", parent)
		}

		parentID = searchResults[0].ID
	}

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
	// Use transaction ID and current timestamp to ensure unique relationship ID
	currentTimestamp := fmt.Sprintf("%s_%s", strings.ReplaceAll(time.Now().Format(time.RFC3339), ":", "-"), uuid.New().String()[:8])
	uniqueRelationshipID := fmt.Sprintf("%s_%s_%s", parentID, createdChild.ID, currentTimestamp)

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
				Key: uniqueRelationshipID,
				Value: models.Relationship{
					RelatedEntityID: createdChild.ID,
					StartTime:       dateISO,
					EndTime:         "",
					ID:              uniqueRelationshipID,
					Name:            relType,
				},
			},
		},
	}

	_, err = c.UpdateEntity(parentID, parentEntity)
	if err != nil {
		return 0, fmt.Errorf("failed to update parent entity: %w", err)
	}

	if isMinisterType(childType) {
		if err := c.ensureMinisterOrgStructure(createdChild.ID, dateISO, ""); err != nil {
			return 0, fmt.Errorf("failed to ensure minister org structure: %w", err)
		}
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

	// Get the parent and child entity IDs based on their types
	var parentID, childID string

	// Handle parent entity retrieval
	if parentType == "president" {
		// Parent is a president - use the helper function
		presidentEntity, err := c.GetPresidentByGovernment(parent, dateISO)
		if err != nil {
			return fmt.Errorf("failed to get parent president entity: %w", err)
		}
		parentID = presidentEntity.ID

	} else if isMinisterType(parentType) {
		// Parent is a minister, need president context to get the correct minister
		presidentName, ok := transaction["president"].(string)
		if !ok || presidentName == "" {
			return fmt.Errorf("president name is required and must be a non-empty string when terminating minister relationships")
		}

		ministerEntity, err := c.GetActiveMinisterByPresident(presidentName, parent, dateISO)
		if err != nil {
			return fmt.Errorf("failed to get parent minister entity: %w", err)
		}
		parentID = ministerEntity.ID

	} else {
		// For other parent types, use the original logic
		parentMajorType := "Organisation"
		if parentType == "citizen" {
			parentMajorType = "Person"
		}
		searchCriteria := &models.SearchCriteria{
			Kind: &models.Kind{
				Major: parentMajorType,
				Minor: parentType,
			},
			Name: parent,
		}

		parentResults, err := c.SearchEntities(searchCriteria)
		if err != nil {
			return fmt.Errorf("failed to search for parent entity: %w", err)
		}
		// Filter for exact name match
		parentResults = utils.FilterByExactName(parentResults, parent)
		if len(parentResults) == 0 {
			return fmt.Errorf("parent entity not found: %s", parent)
		}
		if len(parentResults) > 1 {
			return fmt.Errorf("multiple parent entities found with name '%s'", parent)
		}
		parentID = parentResults[0].ID
	}

	// Handle child entity retrieval
	if isMinisterType(childType) {
		// Child is a minister, parent is the president's name
		presidentName := parent // parent contains the president's name

		ministerEntity, err := c.GetActiveMinisterByPresident(presidentName, child, dateISO)
		if err != nil {
			return fmt.Errorf("failed to get child minister entity: %w", err)
		}
		childID = ministerEntity.ID

	} else if childType == "department" {
		// Child is a department, need to find it under the correct minister
		presidentName, ok := transaction["president"].(string)
		if !ok || presidentName == "" {
			return fmt.Errorf("president name is required and must be a non-empty string when terminating department relationships")
		}

		// First get the minister that should have this department
		ministerEntity, err := c.GetActiveMinisterByPresident(presidentName, parent, dateISO)
		if err != nil {
			return fmt.Errorf("failed to get minister for department termination: %w", err)
		}

		// Then find the department under this minister
		departmentRelations, err := c.GetRelatedEntities(ministerEntity.ID, &models.Relationship{
			Name: "AS_DEPARTMENT",
		})
		if err != nil {
			return fmt.Errorf("failed to get minister's department relationships: %w", err)
		}

		// Find the department with the matching name
		var foundDepartmentID string
		for _, rel := range departmentRelations {
			if rel.EndTime == "" { // Only active relationships
				departmentResults, err := c.SearchEntities(&models.SearchCriteria{ID: rel.RelatedEntityID})
				if err != nil || len(departmentResults) == 0 {
					continue
				}
				if departmentResults[0].Name == child {
					foundDepartmentID = rel.RelatedEntityID
					break
				}
			}
		}

		if foundDepartmentID == "" {
			return fmt.Errorf("department '%s' not found under minister '%s'", child, parent)
		}
		childID = foundDepartmentID

	} else {
		// For other child types, use the original logic
		childMajorType := "Organisation"
		if childType == "citizen" {
			childMajorType = "Person"
		}

		searchCriteria := &models.SearchCriteria{
			Kind: &models.Kind{
				Major: childMajorType,
				Minor: childType,
			},
			Name: child,
		}
		childResults, err := c.SearchEntities(searchCriteria)
		if err != nil {
			return fmt.Errorf("failed to search for child entity: %w", err)
		}
		if len(childResults) == 0 {
			return fmt.Errorf("child entity not found: %s", child)
		}
		childID = childResults[0].ID
	}

	//If we're terminating a minister, check for active departments
	// NOTE: Removing this to allow moving ministers
	// if childType == "minister" {
	// 	// Get all relationships for the minister
	// 	relations, err := c.GetRelatedEntities(childID, &models.Relationship{
	// 		Name: "AS_DEPARTMENT",
	// 	})
	// 	if err != nil {
	// 		return fmt.Errorf("failed to get minister's relationships: %w", err)
	// 	}

	// 	// fmt.Println("relations: ", relations)

	// 	// Manually filter only active (EndTime == "") relationships
	// 	var activeRelations []models.Relationship
	// 	for _, rel := range relations {
	// 		if rel.EndTime == "" {
	// 			activeRelations = append(activeRelations, rel)
	// 		}
	// 	}

	// 	// Check for active departments
	// 	if len(activeRelations) > 0 {
	// 		return fmt.Errorf("cannot terminate minister with active departments")
	// 	}
	// }

	// Get the specific relationship that is still active (no end date) -> this should give us the relationship(s) active for dateISO
	relations, err := c.GetRelatedEntities(parentID, &models.Relationship{
		RelatedEntityID: childID,
		Name:            relType,
	})
	if err != nil {
		return fmt.Errorf("failed to get relationship: %w", err)
	}

	// FIXME: Is it possible to have more than one active relationship? For orgchart case only it won't happen
	// Find the active relationship (no end time)
	// Manually filter for active relationship (i.e., EndTime == "")
	var activeRel *models.Relationship
	for _, rel := range relations {
		if rel.EndTime == "" {
			activeRel = &rel
			break // stop at the first active one
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

	// If we're terminating a minister, also terminate all active role assignments
	// under the minister and secretary org-structure nodes.
	if isMinisterType(childType) {
		ministerNodeID, err := roleNodeID(childID, "minister")
		if err != nil {
			return fmt.Errorf("failed to resolve minister org-structure role node id: %w", err)
		}
		secretaryNodeID, err := roleNodeID(childID, "secretary")
		if err != nil {
			return fmt.Errorf("failed to resolve secretary org-structure role node id: %w", err)
		}

		if err := c.terminateIncomingASRoles(ministerNodeID, dateISO); err != nil {
			return fmt.Errorf("failed to terminate role assignments on minister node: %w", err)
		}
		if err := c.terminateIncomingASRoles(secretaryNodeID, dateISO); err != nil {
			return fmt.Errorf("failed to terminate role assignments on secretary node: %w", err)
		}
	}
	// Ministers, government, and any future root using {id}_org: no-op if no AS_ORGANISATION exists.
	if err := c.terminateOrganisationStructureRelationship(childID, dateISO); err != nil {
		return fmt.Errorf("failed to terminate AS_ORGANISATION: %w", err)
	}

	return nil
}
