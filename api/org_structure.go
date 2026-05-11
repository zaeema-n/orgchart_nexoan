package api

import (
	"fmt"

	"orgchart_nexoan/models"

	"github.com/google/uuid"
)

// ensureMinisterOrgStructure creates the fixed org structure for a minister:
// minister --AS_ORGANISATION--> org <--IS_UNDER-- ministerNode <--IS_UNDER-- secretaryNode
//
// Idempotency rule:
// if AS_ORGANISATION already exists on the minister, the helper returns without changes.
func (c *Client) ensureMinisterOrgStructure(ministerID, start, end string) error {
	existingOrgRels, err := c.GetRelatedEntities(ministerID, &models.Relationship{
		Name:     "AS_ORGANISATION",
		ActiveAt: start,
	})
	if err != nil {
		return fmt.Errorf("failed to check existing AS_ORGANISATION relationship: %w", err)
	}
	if len(existingOrgRels) > 0 {
		return nil
	}

	orgID := fmt.Sprintf("%s_org", ministerID)
	ministerNodeID := fmt.Sprintf("%s_minister", ministerID)
	secretaryNodeID := fmt.Sprintf("%s_secretary", ministerID)

	if err := c.ensureOrgStructureNode(orgID, "Organisation", start, end); err != nil {
		return err
	}
	if err := c.ensureOrgStructureNode(ministerNodeID, "Minister", start, end); err != nil {
		return err
	}
	if err := c.ensureOrgStructureNode(secretaryNodeID, "Secretary", start, end); err != nil {
		return err
	}

	if err := c.addRelationshipIfMissing(ministerID, orgID, "AS_ORGANISATION", start, end); err != nil {
		return err
	}
	if err := c.addRelationshipIfMissing(ministerNodeID, orgID, "IS_UNDER", start, end); err != nil {
		return err
	}
	if err := c.addRelationshipIfMissing(secretaryNodeID, ministerNodeID, "IS_UNDER", start, end); err != nil {
		return err
	}

	return nil
}

// terminateOrganisationStructureRelationship ends every active AS_ORGANISATION edge from rootEntityID
// to its org-structure root node ({rootEntityID}_org). Used for ministers and government.
// Queries relationships active at dateISO (ActiveAt). If none exist, this is a no-op.
func (c *Client) terminateOrganisationStructureRelationship(rootEntityID, dateISO string) error {
	orgID := fmt.Sprintf("%s_org", rootEntityID)
	rels, err := c.GetRelatedEntities(rootEntityID, &models.Relationship{
		Name:            "AS_ORGANISATION",
		RelatedEntityID: orgID,
		ActiveAt:        dateISO,
	})
	if err != nil {
		return fmt.Errorf("failed to get AS_ORGANISATION relationships for entity %s: %w", rootEntityID, err)
	}
	for _, rel := range rels {
		if err := c.terminateRelationship(rootEntityID, rel.ID, dateISO); err != nil {
			return fmt.Errorf("failed to terminate AS_ORGANISATION for entity %s: %w", rootEntityID, err)
		}
	}
	return nil
}

func governmentRoleNodeID(governmentID, role string) (string, error) {
	switch role {
	case "president":
		return fmt.Sprintf("%s_president", governmentID), nil
	case "prime_minister":
		return fmt.Sprintf("%s_prime_minister", governmentID), nil
	default:
		return "", fmt.Errorf("invalid government role '%s': expected 'president' or 'prime_minister'", role)
	}
}

func (c *Client) ensureGovernmentOrgStructure(governmentID, start, end string) error {
	existingOrgRels, err := c.GetRelatedEntities(governmentID, &models.Relationship{
		Name:     "AS_ORGANISATION",
		ActiveAt: start,
	})
	if err != nil {
		return fmt.Errorf("failed to check existing AS_ORGANISATION relationship for government: %w", err)
	}
	if len(existingOrgRels) > 0 {
		return nil
	}

	orgID := fmt.Sprintf("%s_org", governmentID)
	presidentNodeID, err := governmentRoleNodeID(governmentID, "president")
	if err != nil {
		return err
	}
	primeMinisterNodeID, err := governmentRoleNodeID(governmentID, "prime_minister")
	if err != nil {
		return err
	}

	if err := c.ensureOrgStructureNode(orgID, "Organisation", start, end); err != nil {
		return err
	}
	if err := c.ensureOrgStructureNode(presidentNodeID, "President", start, end); err != nil {
		return err
	}
	if err := c.ensureOrgStructureNode(primeMinisterNodeID, "Prime Minister", start, end); err != nil {
		return err
	}

	if err := c.addRelationshipIfMissing(governmentID, orgID, "AS_ORGANISATION", start, end); err != nil {
		return err
	}
	if err := c.addRelationshipIfMissing(presidentNodeID, orgID, "IS_UNDER", start, end); err != nil {
		return err
	}
	if err := c.addRelationshipIfMissing(primeMinisterNodeID, presidentNodeID, "IS_UNDER", start, end); err != nil {
		return err
	}

	return nil
}

func (c *Client) ensureOrgStructureNode(nodeID, name, start, end string) error {
	results, err := c.SearchEntities(&models.SearchCriteria{ID: nodeID})
	if err != nil {
		return fmt.Errorf("failed to check existing org structure node '%s': %w", nodeID, err)
	}
	if len(results) > 0 {
		return nil
	}

	_, err = c.CreateEntity(&models.Entity{
		ID: nodeID,
		Kind: models.Kind{
			Major: "Organisation",
			Minor: "orgStructure",
		},
		Created:    start,
		Terminated: end,
		Name: models.TimeBasedValue{
			StartTime: start,
			Value:     name,
		},
		Metadata:      []models.MetadataEntry{},
		Attributes:    []models.AttributeEntry{},
		Relationships: []models.RelationshipEntry{},
	})
	if err != nil {
		return fmt.Errorf("failed to create org structure node '%s': %w", nodeID, err)
	}

	return nil
}

func (c *Client) addRelationshipIfMissing(sourceID, targetID, relName, start, end string) error {
	existing, err := c.GetRelatedEntities(sourceID, &models.Relationship{
		RelatedEntityID: targetID,
		Name:            relName,
		ActiveAt:        start,
	})
	if err != nil {
		return fmt.Errorf("failed to check existing relationship %s (%s -> %s): %w", relName, sourceID, targetID, err)
	}
	if len(existing) > 0 {
		return nil
	}

	relID := fmt.Sprintf("%s_%s_%s", sourceID, targetID, uuid.New().String())

	_, err = c.UpdateEntity(sourceID, &models.Entity{
		ID: sourceID,
		Relationships: []models.RelationshipEntry{
			{
				Key: relID,
				Value: models.Relationship{
					RelatedEntityID: targetID,
					StartTime:       start,
					EndTime:         end,
					ID:              relID,
					Name:            relName,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create relationship %s (%s -> %s): %w", relName, sourceID, targetID, err)
	}

	return nil
}
