package api

import (
	"fmt"
	"strings"
	"time"

	"orgchart_nexoan/models"

	"github.com/google/uuid"
)

func parseDateISO(dateStr string) (string, error) {
	date, err := time.Parse("2006-01-02", strings.TrimSpace(dateStr))
	if err != nil {
		return "", fmt.Errorf("failed to parse date: %w", err)
	}
	return date.Format(time.RFC3339), nil
}

func roleNodeID(ministerID, role string) (string, error) {
	switch role {
	case "minister":
		return fmt.Sprintf("%s_minister", ministerID), nil
	case "secretary":
		return fmt.Sprintf("%s_secretary", ministerID), nil
	default:
		return "", fmt.Errorf("invalid role '%s': expected 'minister' or 'secretary'", role)
	}
}

func (c *Client) createASRole(personID, targetNodeID, startTime string) error {
	roleRelID := fmt.Sprintf("%s_%s_%s", personID, targetNodeID, uuid.New().String())
	_, err := c.UpdateEntity(personID, &models.Entity{
		ID:         personID,
		Metadata:   []models.MetadataEntry{},
		Attributes: []models.AttributeEntry{},
		Relationships: []models.RelationshipEntry{
			{
				Key: roleRelID,
				Value: models.Relationship{
					RelatedEntityID: targetNodeID,
					Name:            "AS_ROLE",
					StartTime:       startTime,
					EndTime:         "",
					ID:              roleRelID,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add AS_ROLE relationship: %w", err)
	}
	return nil
}

func (c *Client) ensureRoleNodeHasNoActiveAssignment(targetNodeID, dateISO string) error {
	existingRoleRels, err := c.GetRelatedEntities(targetNodeID, &models.Relationship{
		Name:      "AS_ROLE",
		ActiveAt:  dateISO,
		Direction: "INCOMING",
	})
	if err != nil {
		return fmt.Errorf("failed to check existing active role assignments for node '%s': %w", targetNodeID, err)
	}
	if len(existingRoleRels) > 0 {
		return fmt.Errorf("role node '%s' already has an active assignment at %s", targetNodeID, dateISO)
	}

	return nil
}

func (c *Client) terminateRelationship(entityID, relationshipID, dateISO string) error {
	_, err := c.UpdateEntity(entityID, &models.Entity{
		ID: entityID,
		Relationships: []models.RelationshipEntry{
			{
				Key: relationshipID,
				Value: models.Relationship{
					EndTime: dateISO,
					ID:      relationshipID,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to terminate relationship '%s': %w", relationshipID, err)
	}
	return nil
}

func (c *Client) moveIncomingASRoles(oldNodeID, newNodeID, dateISO string) error {
	incomingRoleRels, err := c.GetRelatedEntities(oldNodeID, &models.Relationship{
		Name:      "AS_ROLE",
		Direction: "INCOMING",
	})
	if err != nil {
		return fmt.Errorf("failed to get incoming AS_ROLE relationships for '%s': %w", oldNodeID, err)
	}

	for _, rel := range incomingRoleRels {
		if rel.EndTime != "" {
			continue
		}
		personID := rel.RelatedEntityID
		if err := c.createASRole(personID, newNodeID, dateISO); err != nil {
			return err
		}
		if err := c.terminateRelationship(personID, rel.ID, dateISO); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) terminateIncomingASRoles(nodeID, dateISO string) error {
	incomingRoleRels, err := c.GetRelatedEntities(nodeID, &models.Relationship{
		Name:      "AS_ROLE",
		Direction: "INCOMING",
	})
	if err != nil {
		return fmt.Errorf("failed to get incoming AS_ROLE relationships for '%s': %w", nodeID, err)
	}

	for _, rel := range incomingRoleRels {
		if rel.EndTime != "" {
			continue
		}
		personID := rel.RelatedEntityID
		if err := c.terminateRelationship(personID, rel.ID, dateISO); err != nil {
			return err
		}
	}

	return nil
}
