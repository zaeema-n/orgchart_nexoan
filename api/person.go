package api

import (
	"fmt"
	"strings"
	"time"

	"orgchart_nexoan/internal/utils"
	"orgchart_nexoan/models"
)

// AddPersonEntity creates a new person entity and establishes its relationship with a parent entity.
// Assumes the parent entity already exists.
func (c *Client) AddPersonEntity(transaction map[string]interface{}, entityCounters map[string]int) (int, error) {
	parent := transaction["parent"].(string)
	child := transaction["child"].(string)
	dateStr := transaction["date"].(string)
	parentType := transaction["parent_type"].(string)
	childType := transaction["child_type"].(string)
	transactionID := transaction["transaction_id"].(string)

	var presidentName string
	var role string

	switch {
	case isMinisterType(parentType):
		var ok bool
		presidentName, ok = transaction["president"].(string)
		if !ok || presidentName == "" {
			return 0, fmt.Errorf("president name is required and must be a non-empty string when adding a person to a minister")
		}
		role, ok = transaction["role"].(string)
		if !ok || role == "" {
			return 0, fmt.Errorf("role is required and must be either 'minister' or 'secretary' when adding a person to a minister")
		}
	case parentType == "government":
		var ok bool
		role, ok = transaction["role"].(string)
		if !ok || role == "" {
			return 0, fmt.Errorf("role is required and must be either 'president' or 'prime_minister' when adding a person to government")
		}
	default:
		return 0, fmt.Errorf("adding a person is only supported for minister or government parents")
	}

	date, err := time.Parse("2006-01-02", strings.TrimSpace(dateStr))
	if err != nil {
		return 0, fmt.Errorf("failed to parse date: %w", err)
	}
	dateISO := date.Format(time.RFC3339)

	var parentID string

	switch {
	case isMinisterType(parentType):
		ministerEntity, err := c.GetActiveMinisterByPresident(presidentName, parent, dateISO)
		if err != nil {
			return 0, fmt.Errorf("failed to get parent minister entity: %w", err)
		}
		parentID = ministerEntity.ID
	case parentType == "government":
		parentID, err = c.getGovernmentEntityIDByName(parent)
		if err != nil {
			return 0, err
		}
	default:
		return 0, fmt.Errorf("adding a person is only supported for minister or government parents")
	}

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
	personResults = utils.FilterByExactName(personResults, child)
	if len(personResults) > 1 {
		return 0, fmt.Errorf("multiple entities found for person: %s", child)
	}

	var childID string
	if len(personResults) == 1 {
		childID = personResults[0].ID
	} else {
		if _, exists := entityCounters[childType]; !exists {
			return 0, fmt.Errorf("unknown child type: %s", childType)
		}

		prefixPart := strings.Split(transactionID, "_")[0]
		prefix := fmt.Sprintf("%s_%s", prefixPart, strings.ToLower(childType[:3]))
		entityCounters[childType]++
		newEntityID := fmt.Sprintf("%s_%d", prefix, entityCounters[childType])

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

		createdChild, err := c.CreateEntity(childEntity)
		if err != nil {
			return 0, fmt.Errorf("failed to create child entity: %w", err)
		}
		childID = createdChild.ID
	}

	switch {
	case isMinisterType(parentType):
		targetNodeID, err := roleNodeID(parentID, role)
		if err != nil {
			return 0, err
		}
		if err := c.createASRole(childID, targetNodeID, dateISO); err != nil {
			return 0, err
		}
	case parentType == "government":
		if err := c.ensureGovernmentOrgStructure(parentID, dateISO, ""); err != nil {
			return 0, err
		}
		targetNodeID, err := governmentRoleNodeID(parentID, role)
		if err != nil {
			return 0, err
		}
		if err := c.createASRole(childID, targetNodeID, dateISO); err != nil {
			return 0, err
		}
	}

	return entityCounters[childType], nil
}

// TerminatePersonEntity ends an AS_ROLE edge from the person to the minister's role node (minister or secretary) at the given date.
func (c *Client) TerminatePersonEntity(transaction map[string]interface{}) error {
	// Extract details from the transaction
	parent := transaction["parent"].(string)
	child := transaction["child"].(string)
	dateStr := transaction["date"].(string)
	parentType := transaction["parent_type"].(string)
	childType := transaction["child_type"].(string)

	role, ok := transaction["role"].(string)
	if !ok || role == "" {
		return fmt.Errorf("role is required when terminating a person assignment")
	}

	dateISO, err := parseDateISO(dateStr)
	if err != nil {
		return err
	}

	if childType == "" {
		childType = "citizen"
	}

	// First, find the person (child) entity
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
	// Filter for exact name match
	childResults = utils.FilterByExactName(childResults, child)
	if len(childResults) == 0 {
		return fmt.Errorf("child entity not found: %s", child)
	}
	if len(childResults) > 1 {
		return fmt.Errorf("multiple child entities found with name '%s'", child)
	}
	childID := childResults[0].ID

	var targetNodeID string
	switch {
	case isMinisterType(parentType):
		presidentName, ok := transaction["president"].(string)
		if !ok || presidentName == "" {
			return fmt.Errorf("president name is required and must be a non-empty string when terminating a person under a minister")
		}
		ministerEntity, err := c.GetActiveMinisterByPresident(presidentName, parent, dateISO)
		if err != nil {
			return fmt.Errorf("failed to get parent minister entity: %w", err)
		}
		targetNodeID, err = roleNodeID(ministerEntity.ID, role)
		if err != nil {
			return err
		}
	case parentType == "government":
		governmentID, err := c.getGovernmentEntityIDByName(parent)
		if err != nil {
			return err
		}
		if err := c.ensureGovernmentOrgStructure(governmentID, dateISO, ""); err != nil {
			return err
		}
		targetNodeID, err = governmentRoleNodeID(governmentID, role)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("terminating a person is only supported when the parent is a minister or government")
	}

	// Relations endpoint: AS_ROLE to this role node, active at termination date.
	roleRels, err := c.GetRelatedEntities(childID, &models.Relationship{
		Name:            "AS_ROLE",
		RelatedEntityID: targetNodeID,
		ActiveAt:        dateISO,
	})
	if err != nil {
		return fmt.Errorf("failed to get AS_ROLE relationships for terminate: %w", err)
	}
	if len(roleRels) == 0 {
		return fmt.Errorf("no AS_ROLE relationship active for person '%s' under parent '%s' role '%s' at %s", child, parent, role, dateISO)
	}
	for _, rel := range roleRels {
		if err := c.terminateRelationship(childID, rel.ID, dateISO); err != nil {
			return err
		}
	}
	return nil
}

// MovePerson moves a person's AS_ROLE edge between minister portfolio role nodes:
// role "minister" uses each minister's *_minister node; "secretary" uses *_secretary.
func (c *Client) MovePerson(transaction map[string]interface{}) error {
	newParent := transaction["new_parent"].(string)
	oldParent := transaction["old_parent"].(string)
	child := transaction["child"].(string)
	dateStr := transaction["date"].(string)

	childType := "citizen"
	if v, ok := transaction["child_type"].(string); ok && v != "" {
		childType = v
	}

	dateISO, err := parseDateISO(dateStr)
	if err != nil {
		return err
	}

	childResults, err := c.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Person",
			Minor: childType,
		},
		Name: child,
	})
	if err != nil {
		return fmt.Errorf("failed to search for child entity: %w", err)
	}
	childResults = utils.FilterByExactName(childResults, child)
	if len(childResults) == 0 {
		return fmt.Errorf("child entity not found: %s", child)
	}
	if len(childResults) > 1 {
		return fmt.Errorf("multiple child entities found with name '%s'", child)
	}
	childID := childResults[0].ID

	presidentName, ok := transaction["president"].(string)
	if !ok || presidentName == "" {
		return fmt.Errorf("president name is required and must be a non-empty string")
	}

	role, ok := transaction["role"].(string)
	if !ok || role == "" {
		return fmt.Errorf("role is required and must be either 'minister' or 'secretary' when moving a person between ministers")
	}

	newParentEntity, err := c.GetActiveMinisterByPresident(presidentName, newParent, dateISO)
	if err != nil {
		return fmt.Errorf("failed to get new parent entity: %w", err)
	}
	oldParentEntity, err := c.GetActiveMinisterByPresident(presidentName, oldParent, dateISO)
	if err != nil {
		return fmt.Errorf("failed to get old parent entity: %w", err)
	}

	oldTargetNodeID, err := roleNodeID(oldParentEntity.ID, role)
	if err != nil {
		return err
	}
	newTargetNodeID, err := roleNodeID(newParentEntity.ID, role)
	if err != nil {
		return err
	}

	roleRels, err := c.GetRelatedEntities(childID, &models.Relationship{
		Name:            "AS_ROLE",
		RelatedEntityID: oldTargetNodeID,
		ActiveAt:        dateISO,
	})
	if err != nil {
		return fmt.Errorf("failed to get AS_ROLE relationships for move: %w", err)
	}
	if len(roleRels) == 0 {
		return fmt.Errorf("no AS_ROLE to '%s' role slot active for person '%s' under minister '%s' at %s", role, child, oldParent, dateISO)
	}

	for _, rel := range roleRels {
		if err := c.terminateRelationship(childID, rel.ID, dateISO); err != nil {
			return err
		}
	}
	if err := c.createASRole(childID, newTargetNodeID, dateISO); err != nil {
		return err
	}

	return nil
}
