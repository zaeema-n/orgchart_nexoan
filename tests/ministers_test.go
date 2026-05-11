package tests

import (
	"fmt"
	"orgchart_nexoan/api"
	"orgchart_nexoan/internal/utils"
	"orgchart_nexoan/models"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var client *api.Client

func TestMain(m *testing.M) {
	// Set up test environment with correct URLs
	client = api.NewClient("http://localhost:8080/entities", "http://localhost:8081/v1/entities")

	// Create government node using CreateGovernmentNode
	government, err := client.CreateGovernmentNode()
	if err != nil {
		fmt.Printf("Failed to create government node: %v\n", err)
		os.Exit(1)
	}
	if government == nil {
		fmt.Println("Government node is nil")
		os.Exit(1)
	}
	fmt.Printf("Successfully created government node with ID: %s\n", government.ID)

	// Create president node
	entityCounters := map[string]int{"citizen": 0}
	presidentTransaction := map[string]interface{}{
		"parent":         "Government of Sri Lanka",
		"child":          "Ranil Wickremesinghe",
		// Use an early baseline date so downstream fixture dates are chronologically valid.
		"date":           "2018-01-01",
		"parent_type":    "government",
		"child_type":     "citizen",
		"role":           "president",
		"transaction_id": "2152-12_tr_01",
	}
	_, err = client.AddPersonEntity(presidentTransaction, entityCounters)
	if err != nil {
		fmt.Printf("Failed to create president node: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Successfully created president node: Ranil Wickremesinghe")

	// Run tests
	code := m.Run()
	os.Exit(code)
}

func assertMinisterOrgStructure(t *testing.T, ministerID string) {
	t.Helper()

	orgID := fmt.Sprintf("%s_org", ministerID)
	ministerNodeID := fmt.Sprintf("%s_minister", ministerID)
	secretaryNodeID := fmt.Sprintf("%s_secretary", ministerID)

	asOrgRels, err := client.GetRelatedEntities(ministerID, &models.Relationship{
		RelatedEntityID: orgID,
		Name:            "AS_ORGANISATION",
	})
	assert.NoError(t, err)
	assert.Len(t, asOrgRels, 1, "minister should have exactly one AS_ORGANISATION relationship to org node")

	orgNodeResults, err := client.SearchEntities(&models.SearchCriteria{ID: orgID})
	assert.NoError(t, err)
	assert.Len(t, orgNodeResults, 1, "org node should exist")

	ministerNodeResults, err := client.SearchEntities(&models.SearchCriteria{ID: ministerNodeID})
	assert.NoError(t, err)
	assert.Len(t, ministerNodeResults, 1, "minister org-structure node should exist")

	secretaryNodeResults, err := client.SearchEntities(&models.SearchCriteria{ID: secretaryNodeID})
	assert.NoError(t, err)
	assert.Len(t, secretaryNodeResults, 1, "secretary org-structure node should exist")

	ministerUnderOrgRels, err := client.GetRelatedEntities(ministerNodeID, &models.Relationship{
		RelatedEntityID: orgID,
		Name:            "IS_UNDER",
	})
	assert.NoError(t, err)
	assert.Len(t, ministerUnderOrgRels, 1, "minister node should be IS_UNDER org node")

	secretaryUnderMinisterRels, err := client.GetRelatedEntities(secretaryNodeID, &models.Relationship{
		RelatedEntityID: ministerNodeID,
		Name:            "IS_UNDER",
	})
	assert.NoError(t, err)
	assert.Len(t, secretaryUnderMinisterRels, 1, "secretary node should be IS_UNDER minister node")
}

func TestCreateMinisters(t *testing.T) {
	// Initialize entity counters
	entityCounters := map[string]int{
		"minister": 0,
	}

	// Test cases for creating ministers
	testCases := []struct {
		transactionID string
		parent        string
		parentType    string
		child         string
		childType     string
		relType       string
		date          string
		president     string
	}{
		{
			transactionID: "2153-12_tr_01",
			parent:        "Ranil Wickremesinghe",
			parentType:    "citizen",
			child:         "Minister of Defence",
			childType:     "cabinetMinister",
			relType:       "AS_MINISTER",
			date:          "2019-12-10",
		},
		{
			transactionID: "2153-12_tr_02",
			parent:        "Ranil Wickremesinghe",
			parentType:    "citizen",
			child:         "Minister of Finance, Economic and Policy Development",
			childType:     "cabinetMinister",
			relType:       "AS_MINISTER",
			date:          "2019-12-10"},
	}

	// Create each minister
	for _, tc := range testCases {
		t.Logf("Creating minister: %s", tc.child)

		// Create transaction map for AddEntity
		transaction := map[string]interface{}{
			"parent":         tc.parent,
			"child":          tc.child,
			"date":           tc.date,
			"parent_type":    tc.parentType,
			"child_type":     tc.childType,
			"rel_type":       tc.relType,
			"transaction_id": tc.transactionID}

		// Use AddEntity to create the minister
		_, err := client.AddOrgEntity(transaction, entityCounters)
		assert.NoError(t, err)

		// Update the counter for the next iteration
		entityCounters["minister"]++

		// Verify the minister was created by searching for it
		searchCriteria := &models.SearchCriteria{
			Kind: &models.Kind{
				Major: "Organisation",
				Minor: tc.childType,
			},
			Name: tc.child,
		}

		results, err := client.SearchEntities(searchCriteria)
		assert.NoError(t, err)
		results = utils.FilterByExactName(results, tc.child)
		assert.Len(t, results, 1)
		assert.Equal(t, tc.child, results[0].Name)
		assertMinisterOrgStructure(t, results[0].ID)

		// Verify the relationship was created by checking parent's relationships
		parentResults, err := client.SearchEntities(&models.SearchCriteria{
			Kind: &models.Kind{
				Major: "Person",
				Minor: tc.parentType,
			},
			Name: tc.parent,
		})
		assert.NoError(t, err)
		parentResults = utils.FilterByExactName(parentResults, tc.parent)
		assert.Len(t, parentResults, 1)

		// Get parent's metadata to verify relationship
		// metadata, err := client.GetEntityMetadata(parentResults[0].ID)
		// assert.NoError(t, err)
		// assert.NotNil(t, metadata)
	}
}

func TestCreateDepartments(t *testing.T) {
	// Initialize entity counters
	entityCounters := map[string]int{
		"department": 0,
	}

	// Test cases for creating departments
	testCases := []struct {
		transactionID string
		parent        string
		parentType    string
		child         string
		childType     string
		relType       string
		date          string
		president     string
	}{
		{
			transactionID: "2153-12_tr_03",
			parent:        "Minister of Defence",
			parentType:    "cabinetMinister",
			child:         "Sri Lankan Army",
			childType:     "department",
			relType:       "AS_DEPARTMENT",
			date:          "2019-12-10",
			president:     "Ranil Wickremesinghe",
		},
		{
			transactionID: "2153-12_tr_04",
			parent:        "Minister of Finance, Economic and Policy Development",
			parentType:    "cabinetMinister",
			child:         "Department of Taxes",
			childType:     "department",
			relType:       "AS_DEPARTMENT",
			date:          "2019-12-10",
			president:     "Ranil Wickremesinghe",
		},
		{
			transactionID: "2153-12_tr_05",
			parent:        "Minister of Finance, Economic and Policy Development",
			parentType:    "cabinetMinister",
			child:         "Department of Policies",
			childType:     "department",
			relType:       "AS_DEPARTMENT",
			date:          "2019-12-10",
			president:     "Ranil Wickremesinghe",
		},
	}

	// Create each department
	for _, tc := range testCases {
		t.Logf("Creating department: %s under minister: %s", tc.child, tc.parent)

		// Create transaction map for AddEntity
		transaction := map[string]interface{}{
			"parent":         tc.parent,
			"child":          tc.child,
			"date":           tc.date,
			"parent_type":    tc.parentType,
			"child_type":     tc.childType,
			"rel_type":       tc.relType,
			"transaction_id": tc.transactionID,
			"president":      tc.president,
		}

		// Use AddEntity to create the department
		_, err := client.AddOrgEntity(transaction, entityCounters)
		assert.NoError(t, err)

		// Update the counter for the next iteration
		entityCounters[tc.childType]++

		// Verify the department was created by searching for it
		searchCriteria := &models.SearchCriteria{
			Kind: &models.Kind{
				Major: "Organisation",
				Minor: tc.childType,
			},
			Name: tc.child,
		}

		results, err := client.SearchEntities(searchCriteria)
		assert.NoError(t, err)
		results = utils.FilterByExactName(results, tc.child)
		assert.Len(t, results, 1)
		assert.Equal(t, tc.child, results[0].Name)

		// Verify the relationship was created by checking minister's relationships
		ministerResults, err := client.SearchEntities(&models.SearchCriteria{
			Kind: &models.Kind{
				Major: "Organisation",
				Minor: tc.parentType,
			},
			Name: tc.parent,
		})
		assert.NoError(t, err)
		ministerResults = utils.FilterByExactName(ministerResults, tc.parent)
		assert.Len(t, ministerResults, 1)

		// Get minister's relationships to verify department relationship
		// relations, err := client.GetRelatedEntities(ministerResults[0].ID, &models.Relationship{
		// 	Name:    tc.relType,
		// 	EndTime: "",
		// })
		// assert.NoError(t, err)
		// assert.NotEmpty(t, relations, "Minister should have at least one department relationship")
	}
}

func TestTerminateDepartment(t *testing.T) {
	// Create transaction map for terminating the department
	transaction := map[string]interface{}{
		"parent":      "Minister of Defence",
		"child":       "Sri Lankan Army",
		"date":        "2024-01-01",
		"parent_type": "cabinetMinister",
		"child_type":  "department",
		"rel_type":    "AS_DEPARTMENT",
		"president":   "Ranil Wickremesinghe",
	}

	// Terminate the department relationship
	err := client.TerminateOrgEntity(transaction)
	assert.NoError(t, err)

	// Find the minister to verify the relationship
	ministerResults, err := client.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "cabinetMinister",
		},
		Name: "Minister of Defence",
	})
	assert.NoError(t, err)
	ministerResults = utils.FilterByExactName(ministerResults, "Minister of Defence")
	assert.Len(t, ministerResults, 1)
	ministerID := ministerResults[0].ID

	// Find the department
	departmentResults, err := client.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "department",
		},
		Name: "Sri Lankan Army",
	})
	assert.NoError(t, err)
	departmentResults = utils.FilterByExactName(departmentResults, "Sri Lankan Army")
	assert.Len(t, departmentResults, 1)
	departmentID := departmentResults[0].ID

	// Verify the relationship is terminated
	relations, err := client.GetRelatedEntities(ministerID, &models.Relationship{
		RelatedEntityID: departmentID,
		Name:            "AS_DEPARTMENT",
	})
	assert.NoError(t, err)
	assert.Len(t, relations, 1, "Should find one relationship")
	assert.Equal(t, "2024-01-01T00:00:00Z", relations[0].EndTime, "Relationship should be terminated")
}

func TestTerminateMinister(t *testing.T) {
	// Create transaction map for terminating the minister
	transaction := map[string]interface{}{
		"parent":      "Ranil Wickremesinghe",
		"child":       "Minister of Defence",
		"date":        "2024-01-01",
		"parent_type": "citizen",
		"child_type":  "cabinetMinister",
		"rel_type":    "AS_MINISTER",
	}

	// Terminate the minister relationship
	err := client.TerminateOrgEntity(transaction)
	assert.NoError(t, err)

	// Find the president to verify the role assignment under government.
	presResults, err := client.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Person",
			Minor: "citizen",
		},
		Name: "Ranil Wickremesinghe",
	})
	assert.NoError(t, err)
	presResults = utils.FilterByExactName(presResults, "Ranil Wickremesinghe")
	assert.Len(t, presResults, 1)
	presID := presResults[0].ID

	// Get government node to check AS_ROLE relationship to the president role node.
	governmentResults, err := client.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "government",
		},
	})
	assert.NoError(t, err)
	assert.Len(t, governmentResults, 1)
	presidentRoleNodeID := governmentResults[0].ID + "_president"

	// Verify this citizen has AS_ROLE relationship to the government president role node.
	presidentRelations, err := client.GetRelatedEntities(presID, &models.Relationship{
		Name:            "AS_ROLE",
		RelatedEntityID: presidentRoleNodeID,
	})
	assert.NoError(t, err)
	assert.Len(t, presidentRelations, 1, "Should find AS_ROLE relationship to the government president role node")
	assert.Equal(t, "", presidentRelations[0].EndTime, "President AS_ROLE relationship should be active")

	// Find the minister
	ministerResults, err := client.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "cabinetMinister",
		},
		Name: "Minister of Defence",
	})
	assert.NoError(t, err)
	ministerResults = utils.FilterByExactName(ministerResults, "Minister of Defence")
	assert.Len(t, ministerResults, 1)
	ministerID := ministerResults[0].ID

	// Verify the relationship is terminated
	relations, err := client.GetRelatedEntities(presID, &models.Relationship{
		RelatedEntityID: ministerID,
		Name:            "AS_MINISTER",
	})
	assert.NoError(t, err)
	assert.Len(t, relations, 1, "Should find one relationship")
	assert.Equal(t, "2024-01-01T00:00:00Z", relations[0].EndTime, "Relationship should be terminated")

	orgID := fmt.Sprintf("%s_org", ministerID)
	asOrgRels, err := client.GetRelatedEntities(ministerID, &models.Relationship{
		RelatedEntityID: orgID,
		Name:            "AS_ORGANISATION",
	})
	assert.NoError(t, err)
	assert.Len(t, asOrgRels, 1, "Should find AS_ORGANISATION to org node")
	assert.Equal(t, "2024-01-01T00:00:00Z", asOrgRels[0].EndTime, "AS_ORGANISATION should be terminated with the minister")
}

func TestMoveDepartment(t *testing.T) {
	// First create a new minister
	entityCounters := map[string]int{
		"minister": 2, // Since we already have 2 ministers from previous tests
	}

	// Create transaction map for new minister
	newMinisterTransaction := map[string]interface{}{
		"parent":         "Ranil Wickremesinghe",
		"child":          "Minister of Education",
		"date":           "2024-01-01",
		"parent_type":    "citizen",
		"child_type":     "cabinetMinister",
		"rel_type":       "AS_MINISTER",
		"transaction_id": "2153/12_tr_06",
		"president":      "Ranil Wickremesinghe",
	}

	// Create the new minister
	_, err := client.AddOrgEntity(newMinisterTransaction, entityCounters)
	assert.NoError(t, err)

	// Create transaction map for moving the department
	transaction := map[string]interface{}{
		"old_parent":         "Minister of Finance, Economic and Policy Development",
		"new_parent":         "Minister of Education",
		"child":              "Department of Policies",
		"type":               "department",
		"date":               "2024-01-01",
		"old_president_name": "Ranil Wickremesinghe",
		"new_president_name": "Ranil Wickremesinghe",
	}

	// Move the department
	err = client.MoveDepartment(transaction)
	assert.NoError(t, err)

	// Find the new minister to verify the new relationship
	newMinisterResults, err := client.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "cabinetMinister",
		},
		Name: "Minister of Education",
	})
	assert.NoError(t, err)
	newMinisterResults = utils.FilterByExactName(newMinisterResults, "Minister of Education")
	assert.Len(t, newMinisterResults, 1)
	newMinisterID := newMinisterResults[0].ID

	// Find the department
	departmentResults, err := client.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "department",
		},
		Name: "Department of Policies",
	})
	assert.NoError(t, err)
	departmentResults = utils.FilterByExactName(departmentResults, "Department of Policies")
	assert.Len(t, departmentResults, 1)
	departmentID := departmentResults[0].ID

	// Verify the new relationship exists
	relations, err := client.GetRelatedEntities(newMinisterID, &models.Relationship{
		RelatedEntityID: departmentID,
		Name:            "AS_DEPARTMENT",
	})
	assert.NoError(t, err)

	// Manually filter for active relationships (EndTime == "")
	var activeRelations []models.Relationship
	for _, rel := range relations {
		if rel.EndTime == "" {
			activeRelations = append(activeRelations, rel)
		}
	}

	assert.Len(t, activeRelations, 1, "Should find one active relationship")
	assert.Equal(t, "2024-01-01T00:00:00Z", activeRelations[0].StartTime)

	// Find the old minister to verify the old relationship is terminated
	oldMinisterResults, err := client.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "cabinetMinister",
		},
		Name: "Minister of Finance, Economic and Policy Development",
	})
	assert.NoError(t, err)
	oldMinisterResults = utils.FilterByExactName(oldMinisterResults, "Minister of Finance, Economic and Policy Development")
	assert.Len(t, oldMinisterResults, 1)
	oldMinisterID := oldMinisterResults[0].ID

	// Verify the old relationship is terminated
	oldRelations, err := client.GetRelatedEntities(oldMinisterID, &models.Relationship{
		RelatedEntityID: departmentID,
		Name:            "AS_DEPARTMENT",
	})
	assert.NoError(t, err)
	assert.Len(t, oldRelations, 1, "Should find one relationship")
	assert.Equal(t, "2024-01-01T00:00:00Z", oldRelations[0].EndTime, "Relationship should be terminated")
}

func TestRenameMinister(t *testing.T) {
	// Initialize entity counters
	entityCounters := map[string]int{
		"minister": 2,
	}

	// Create transaction map for renaming the minister
	transaction := map[string]interface{}{
		"old":            "Minister of Finance, Economic and Policy Development",
		"new":            "Minister of Finance",
		"type":           "cabinetMinister",
		"date":           "2024-01-01",
		"transaction_id": "2153-13_tr_01",
		"president":      "Ranil Wickremesinghe",
	}

	// Rename the minister
	newMinisterCounter, err := client.RenameMinister(transaction, entityCounters)
	assert.NoError(t, err)
	assert.Greater(t, newMinisterCounter, 0)

	// Find the new minister to verify it exists
	newMinisterResults, err := client.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "cabinetMinister",
		},
		Name: "Minister of Finance",
	})
	assert.NoError(t, err)
	newMinisterResults = utils.FilterByExactName(newMinisterResults, "Minister of Finance")
	assert.Len(t, newMinisterResults, 1)
	newMinisterID := newMinisterResults[0].ID
	assertMinisterOrgStructure(t, newMinisterID)

	// Find the old minister
	oldMinisterResults, err := client.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "cabinetMinister",
		},
		Name: "Minister of Finance, Economic and Policy Development",
	})
	assert.NoError(t, err)
	oldMinisterResults = utils.FilterByExactName(oldMinisterResults, "Minister of Finance, Economic and Policy Development")
	assert.Len(t, oldMinisterResults, 1)
	oldMinisterID := oldMinisterResults[0].ID

	// Verify the RENAMED_TO relationship exists
	relations, err := client.GetRelatedEntities(oldMinisterID, &models.Relationship{
		RelatedEntityID: newMinisterID,
		Name:            "RENAMED_TO",
	})
	assert.NoError(t, err)

	// Manually filter for active relationships (EndTime == "")
	var activeRelations []models.Relationship
	for _, rel := range relations {
		if rel.EndTime == "" {
			activeRelations = append(activeRelations, rel)
		}
	}

	assert.Len(t, activeRelations, 1)
	assert.Equal(t, "2024-01-01T00:00:00Z", activeRelations[0].StartTime)

	// Verify the old minister's president relationship is terminated
	presidentResults, err := client.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Person",
			Minor: "citizen",
		},
		Name: "Ranil Wickremesinghe",
	})
	assert.NoError(t, err)
	presidentResults = utils.FilterByExactName(presidentResults, "Ranil Wickremesinghe")
	assert.Len(t, presidentResults, 1)
	presidentID := presidentResults[0].ID

	presidentRelations, err := client.GetRelatedEntities(presidentID, &models.Relationship{
		RelatedEntityID: oldMinisterID,
		Name:            "AS_MINISTER",
	})
	assert.NoError(t, err)
	assert.Len(t, presidentRelations, 1)
	assert.Equal(t, "2024-01-01T00:00:00Z", presidentRelations[0].EndTime)

	oldOrgID := fmt.Sprintf("%s_org", oldMinisterID)
	asOrgRels, err := client.GetRelatedEntities(oldMinisterID, &models.Relationship{
		RelatedEntityID: oldOrgID,
		Name:            "AS_ORGANISATION",
	})
	assert.NoError(t, err)
	assert.Len(t, asOrgRels, 1, "old minister should have AS_ORGANISATION to org node")
	assert.Equal(t, "2024-01-01T00:00:00Z", asOrgRels[0].EndTime, "AS_ORGANISATION should end when portfolio is renamed")

	// Verify the new minister has the president relationship
	newPresidentRelations, err := client.GetRelatedEntities(presidentID, &models.Relationship{
		RelatedEntityID: newMinisterID,
		Name:            "AS_MINISTER",
	})
	assert.NoError(t, err)

	// Manually filter for active relationships (EndTime == "")
	var activeNewPresidentRelations []models.Relationship
	for _, rel := range newPresidentRelations {
		if rel.EndTime == "" {
			activeNewPresidentRelations = append(activeNewPresidentRelations, rel)
		}
	}

	assert.Len(t, activeNewPresidentRelations, 1)
	assert.Equal(t, "2024-01-01T00:00:00Z", activeNewPresidentRelations[0].StartTime)

	// Verify all departments were transferred
	oldDeptRelations, err := client.GetRelatedEntities(oldMinisterID, &models.Relationship{Name: "AS_DEPARTMENT"})
	assert.NoError(t, err)
	newDeptRelations, err := client.GetRelatedEntities(newMinisterID, &models.Relationship{Name: "AS_DEPARTMENT"})
	assert.NoError(t, err)

	// Manually filter for active relationships (EndTime == "")
	var activeOldDeptRelations []models.Relationship
	for _, rel := range oldDeptRelations {
		if rel.EndTime == "" {
			activeOldDeptRelations = append(activeOldDeptRelations, rel)
		}
	}

	var activeNewDeptRelations []models.Relationship
	for _, rel := range newDeptRelations {
		if rel.EndTime == "" {
			activeNewDeptRelations = append(activeNewDeptRelations, rel)
		}
	}

	// All departments should be transferred (no active departments for old minister)
	assert.Len(t, activeOldDeptRelations, 0, "Old minister should have no active departments")
	assert.Greater(t, len(activeNewDeptRelations), 0, "New minister should have active departments")
}

func TestRenameDepartment(t *testing.T) {
	// Initialize entity counters
	entityCounters := map[string]int{
		"department": 0,
	}

	var err error

	// Create department
	departmentTransaction := map[string]interface{}{
		"parent":         "Minister of Finance",
		"child":          "National Bank",
		"date":           "2024-02-01",
		"parent_type":    "cabinetMinister",
		"child_type":     "department",
		"rel_type":       "AS_DEPARTMENT",
		"transaction_id": "2153-13_tr_02",
		"president":      "Ranil Wickremesinghe",
	}

	_, err = client.AddOrgEntity(departmentTransaction, entityCounters)
	assert.NoError(t, err)
	entityCounters["department"]++

	// Create transaction map for renaming the department
	transaction := map[string]interface{}{
		"old":            "National Bank",
		"new":            "Department of the National Bank",
		"type":           "department",
		"date":           "2024-02-02",
		"transaction_id": "2153-13_tr_05",
		"president":      "Ranil Wickremesinghe",
	}

	// Rename the department
	newDepartmentCounter, err := client.RenameDepartment(transaction, entityCounters)
	assert.NoError(t, err)
	assert.Greater(t, newDepartmentCounter, 0)

	// Find the new department to verify it exists
	newDepartmentResults, err := client.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "department",
		},
		Name: "Department of the National Bank",
	})
	assert.NoError(t, err)
	newDepartmentResults = utils.FilterByExactName(newDepartmentResults, "Department of the National Bank")
	assert.Len(t, newDepartmentResults, 1)
	newDepartmentID := newDepartmentResults[0].ID

	// Find the old department
	oldDepartmentResults, err := client.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "department",
		},
		Name: "National Bank",
	})
	assert.NoError(t, err)
	oldDepartmentResults = utils.FilterByExactName(oldDepartmentResults, "National Bank")
	assert.Len(t, oldDepartmentResults, 1)
	oldDepartmentID := oldDepartmentResults[0].ID

	// Verify the RENAMED_TO relationship exists
	relations, err := client.GetRelatedEntities(oldDepartmentID, &models.Relationship{
		RelatedEntityID: newDepartmentID,
		Name:            "RENAMED_TO",
	})
	assert.NoError(t, err)

	// Manually filter for active relationships (EndTime == "")
	var activeRelations []models.Relationship
	for _, rel := range relations {
		if rel.EndTime == "" {
			activeRelations = append(activeRelations, rel)
		}
	}
	assert.Len(t, activeRelations, 1)
	assert.Equal(t, "2024-02-02T00:00:00Z", activeRelations[0].StartTime)

	// Find the minister that owns the department
	ministerResults, err := client.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "cabinetMinister",
		},
		Name: "Minister of Finance",
	})
	assert.NoError(t, err)
	ministerResults = utils.FilterByExactName(ministerResults, "Minister of Finance")
	assert.Len(t, ministerResults, 1)
	ministerID := ministerResults[0].ID

	// Verify the old department's minister relationship is terminated
	minRelations, err := client.GetRelatedEntities(ministerID, &models.Relationship{
		RelatedEntityID: oldDepartmentID,
		Name:            "AS_DEPARTMENT",
	})
	assert.NoError(t, err)
	assert.Len(t, minRelations, 1)
	assert.Equal(t, "2024-02-02T00:00:00Z", minRelations[0].EndTime)

	// Verify the new department has the minister relationship
	newMinRelations, err := client.GetRelatedEntities(ministerID, &models.Relationship{
		RelatedEntityID: newDepartmentID,
		Name:            "AS_DEPARTMENT",
	})
	assert.NoError(t, err)

	// Manually filter for active relationships (EndTime == "")
	var activeNewMinRelations []models.Relationship
	for _, rel := range newMinRelations {
		if rel.EndTime == "" {
			activeNewMinRelations = append(activeNewMinRelations, rel)
		}
	}
	assert.Len(t, activeNewMinRelations, 1)
	assert.Equal(t, "2024-02-02T00:00:00Z", activeNewMinRelations[0].StartTime)
}

func TestMergeMinisters(t *testing.T) {
	// Initialize entity counters
	entityCounters := map[string]int{
		"minister": 0, // Since we already have 3 ministers from previous tests
	}

	// Create transaction map for merging ministers
	transaction := map[string]interface{}{
		"old":            "[Minister of Education; Minister of Finance]",
		"new":            "Minister of Finance and Education",
		"date":           "2025-01-01",
		"transaction_id": "2154-13_tr_01",
		"president":      "Ranil Wickremesinghe",
	}

	// Merge the ministers
	newMinisterCounter, err := client.MergeMinisters(transaction, entityCounters)
	assert.NoError(t, err)
	assert.Greater(t, newMinisterCounter, 0)

	// Find the new minister to verify it exists
	newMinisterResults, err := client.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "cabinetMinister",
		},
		Name: "Minister of Finance and Education",
	})
	assert.NoError(t, err)
	newMinisterResults = utils.FilterByExactName(newMinisterResults, "Minister of Finance and Education")
	assert.Len(t, newMinisterResults, 1)
	newMinisterID := newMinisterResults[0].ID
	assertMinisterOrgStructure(t, newMinisterID)

	// Find the old ministers
	oldMinisterResults, err := client.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "cabinetMinister",
		},
		Name: "Minister of Education",
	})
	assert.NoError(t, err)
	oldMinisterResults = utils.FilterByExactName(oldMinisterResults, "Minister of Education")
	assert.Len(t, oldMinisterResults, 1)
	oldMinister1ID := oldMinisterResults[0].ID

	oldMinisterResults, err = client.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "cabinetMinister",
		},
		Name: "Minister of Finance",
	})
	assert.NoError(t, err)
	oldMinisterResults = utils.FilterByExactName(oldMinisterResults, "Minister of Finance")
	assert.Len(t, oldMinisterResults, 1)
	oldMinister2ID := oldMinisterResults[0].ID

	// Verify the MERGED_INTO relationships exist
	oldRelations1, err := client.GetRelatedEntities(oldMinister1ID, &models.Relationship{
		RelatedEntityID: newMinisterID,
		Name:            "MERGED_INTO",
	})
	assert.NoError(t, err)

	// Manually filter for active relationships (EndTime == "")
	var activeOldRelations1 []models.Relationship
	for _, rel := range oldRelations1 {
		if rel.EndTime == "" {
			activeOldRelations1 = append(activeOldRelations1, rel)
		}
	}
	assert.Len(t, activeOldRelations1, 1)
	assert.Equal(t, "2025-01-01T00:00:00Z", activeOldRelations1[0].StartTime)

	oldRelations2, err := client.GetRelatedEntities(oldMinister2ID, &models.Relationship{
		RelatedEntityID: newMinisterID,
		Name:            "MERGED_INTO",
	})
	assert.NoError(t, err)

	// Manually filter for active relationships (EndTime == "")
	var activeOldRelations2 []models.Relationship
	for _, rel := range oldRelations2 {
		if rel.EndTime == "" {
			activeOldRelations2 = append(activeOldRelations2, rel)
		}
	}
	assert.Len(t, activeOldRelations2, 1)
	assert.Equal(t, "2025-01-01T00:00:00Z", activeOldRelations2[0].StartTime)

	// Verify the old ministers' president relationships are terminated
	presidentResults, err := client.SearchEntities(&models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Person",
			Minor: "citizen",
		},
		Name: "Ranil Wickremesinghe",
	})
	assert.NoError(t, err)
	presidentResults = utils.FilterByExactName(presidentResults, "Ranil Wickremesinghe")
	assert.Len(t, presidentResults, 1)
	presidentID := presidentResults[0].ID

	presidentRelations, err := client.GetRelatedEntities(presidentID, &models.Relationship{
		RelatedEntityID: oldMinister1ID,
		Name:            "AS_MINISTER",
	})
	assert.NoError(t, err)
	assert.Len(t, presidentRelations, 1)
	assert.Equal(t, "2025-01-01T00:00:00Z", presidentRelations[0].EndTime)

	presidentRelations, err = client.GetRelatedEntities(presidentID, &models.Relationship{
		RelatedEntityID: oldMinister2ID,
		Name:            "AS_MINISTER",
	})
	assert.NoError(t, err)
	assert.Len(t, presidentRelations, 1)
	assert.Equal(t, "2025-01-01T00:00:00Z", presidentRelations[0].EndTime)

	// Verify the new minister has the president relationship
	newPresidentRelations, err := client.GetRelatedEntities(presidentID, &models.Relationship{
		RelatedEntityID: newMinisterID,
		Name:            "AS_MINISTER",
	})
	assert.NoError(t, err)

	// Manually filter for active relationships (EndTime == "")
	var activeNewPresidentRelations []models.Relationship
	for _, rel := range newPresidentRelations {
		if rel.EndTime == "" {
			activeNewPresidentRelations = append(activeNewPresidentRelations, rel)
		}
	}
	assert.Len(t, activeNewPresidentRelations, 1)
	assert.Equal(t, "2025-01-01T00:00:00Z", activeNewPresidentRelations[0].StartTime)

	// Verify all departments were transferred
	newDeptRelations, err := client.GetRelatedEntities(newMinisterID, &models.Relationship{Name: "AS_DEPARTMENT"})
	assert.NoError(t, err)

	// Manually filter for active relationships (EndTime == "")
	var activeNewDeptRelations []models.Relationship
	for _, rel := range newDeptRelations {
		if rel.EndTime == "" {
			activeNewDeptRelations = append(activeNewDeptRelations, rel)
		}
	}

	// Should have at least 2 departments (one from each old minister)
	assert.GreaterOrEqual(t, len(activeNewDeptRelations), 2, "New minister should have at least 2 active departments")

	// Verify old ministers have no active departments
	oldDeptRelations1, err := client.GetRelatedEntities(oldMinister1ID, &models.Relationship{Name: "AS_DEPARTMENT"})
	assert.NoError(t, err)
	oldDeptRelations2, err := client.GetRelatedEntities(oldMinister2ID, &models.Relationship{Name: "AS_DEPARTMENT"})
	assert.NoError(t, err)

	// Manually filter for active relationships (EndTime == "")
	var activeOldDeptRelations1 []models.Relationship
	for _, rel := range oldDeptRelations1 {
		if rel.EndTime == "" {
			activeOldDeptRelations1 = append(activeOldDeptRelations1, rel)
		}
	}
	var activeOldDeptRelations2 []models.Relationship
	for _, rel := range oldDeptRelations2 {
		if rel.EndTime == "" {
			activeOldDeptRelations2 = append(activeOldDeptRelations2, rel)
		}
	}

	assert.Len(t, activeOldDeptRelations1, 0, "First old minister should have no active departments")
	assert.Len(t, activeOldDeptRelations2, 0, "Second old minister should have no active departments")
}

func TestTerminateNonExistentMinister(t *testing.T) {
	// Create transaction map for terminating a non-existent minister
	transaction := map[string]interface{}{
		"parent":      "Ranil Wickremesinghe",
		"child":       "Non Existent Minister",
		"date":        "2025-01-01",
		"parent_type": "citizen",
		"child_type":  "cabinetMinister",
		"rel_type":    "AS_MINISTER",
	}

	// Attempt to terminate the non-existent minister
	err := client.TerminateOrgEntity(transaction)
	assert.Error(t, err)
}

// func TestTerminateMinisterWithChildren(t *testing.T) {
// 	// First create a minister with a department
// 	entityCounters := map[string]int{
// 		"minister":   0,
// 		"department": 0,
// 	}

// 	// Create minister
// 	ministerTransaction := map[string]interface{}{
// 		"parent":         "Ranil Wickremesinghe",
// 		"child":          "Minister to Terminate",
// 		"date":           "2025-01-01",
// 		"parent_type":    "citizen",
// 		"child_type":     "cabinetMinister",
// 		"rel_type":       "AS_MINISTER",
// 		"transaction_id": "2154-14_tr_01",
// 		"president":      "Ranil Wickremesinghe",
// 	}

// 	_, err := client.AddOrgEntity(ministerTransaction, entityCounters)
// 	assert.NoError(t, err)

// 	// Create department under the minister
// 	departmentTransaction := map[string]interface{}{
// 		"parent":         "Minister to Terminate",
// 		"child":          "Department Under Minister",
// 		"date":           "2025-01-01",
// 		"parent_type":    "cabinetMinister",
// 		"child_type":     "department",
// 		"rel_type":       "AS_DEPARTMENT",
// 		"transaction_id": "2154-14_tr_02",
// 		"president":      "Ranil Wickremesinghe",
// 	}

// 	_, err = client.AddOrgEntity(departmentTransaction, entityCounters)
// 	assert.NoError(t, err)

// 	// Debug: Print minister's relationships before termination
// 	ministerResults, err := client.SearchEntities(&models.SearchCriteria{
// 		Kind: &models.Kind{
// 			Major: "Organisation",
// 			Minor: "cabinetMinister",
// 		},
// 		Name: "Minister to Terminate",
// 	})
// 	assert.NoError(t, err)
// 	ministerResults = utils.FilterByExactName(ministerResults, "Minister to Terminate")
// 	assert.Len(t, ministerResults, 1)
// 	ministerID := ministerResults[0].ID

// 	//fmt.Printf("Debug: Minister ID: %s\n", ministerID)
// 	_, err = client.GetRelatedEntities(ministerID, &models.Relationship{})
// 	assert.NoError(t, err)
// 	//fmt.Printf("Debug: Minister's relationships before termination: %+v\n", relations)

// 	// Attempt to terminate the minister
// 	terminateTransaction := map[string]interface{}{
// 		"parent":      "Ranil Wickremesinghe",
// 		"child":       "Minister to Terminate",
// 		"date":        "2025-01-02",
// 		"parent_type": "citizen",
// 		"child_type":  "cabinetMinister",
// 		"rel_type":    "AS_MINISTER",
// 	}

// 	// fmt.Printf("Debug: Attempting to terminate minister with transaction: %+v\n", terminateTransaction)
// 	err = client.TerminateOrgEntity(terminateTransaction)
// 	assert.Error(t, err)
// 	// assert.Contains(t, err.Error(), "cannot terminate minister with active departments")

// }

func TestMoveDepartmentToNonExistentMinister(t *testing.T) {
	// Create transaction map for moving department to non-existent minister
	transaction := map[string]interface{}{
		"old_parent":         "Minister of Finance and Education",
		"new_parent":         "Non Existent Minister",
		"child":              "Department of Policies",
		"type":               "department",
		"date":               "2025-01-01",
		"new_president_name": "Ranil Wickremesinghe",
		"old_president_name": "Ranil Wickremesinghe",
	}

	// Attempt to move the department
	err := client.MoveDepartment(transaction)
	assert.Error(t, err)
}

func TestMergeNonExistentMinister(t *testing.T) {
	// Initialize entity counters
	entityCounters := map[string]int{
		"minister": 0,
	}

	// Create transaction map for merging non-existent minister
	transaction := map[string]interface{}{
		"old":            "[Non Existent Minister]",
		"new":            "New Merged Minister",
		"date":           "2025-01-01",
		"transaction_id": "2154/14_tr_03",
		"president":      "Ranil Wickremesinghe",
	}

	// Attempt to merge the ministers
	_, err := client.MergeMinisters(transaction, entityCounters)
	//fmt.Printf("Debug: Full error from MergeMinisters: %+v\n", err)
	assert.Error(t, err)
}

func TestCreateDuplicateMinister(t *testing.T) {
	// Initialize entity counters
	entityCounters := map[string]int{
		"minister": 0,
	}

	// Create transaction map for first minister
	firstMinisterTransaction := map[string]interface{}{
		"parent":         "Ranil Wickremesinghe",
		"child":          "Duplicate Minister",
		"date":           "2025-01-01",
		"parent_type":    "citizen",
		"child_type":     "cabinetMinister",
		"rel_type":       "AS_MINISTER",
		"transaction_id": "2154/15_tr_01",
		"president":      "Ranil Wickremesinghe",
	}

	// Create the first minister
	firstMinister, err := client.AddOrgEntity(firstMinisterTransaction, entityCounters)
	assert.NoError(t, err)
	assert.NotNil(t, firstMinister)

	// Update counter for second attempt
	entityCounters["minister"]++

	// Create transaction map for second minister with same name
	secondMinisterTransaction := map[string]interface{}{
		"parent":         "Ranil Wickremesinghe",
		"child":          "Duplicate Minister",
		"date":           "2025-01-02",
		"parent_type":    "citizen",
		"child_type":     "cabinetMinister",
		"rel_type":       "AS_MINISTER",
		"transaction_id": "2154/15_tr_02",
		"president":      "Ranil Wickremesinghe",
	}

	// Create the second minister with same name
	secondMinister, err := client.AddOrgEntity(secondMinisterTransaction, entityCounters)
	assert.NoError(t, err, "Should be able to create minister with same name but different ID")
	assert.NotNil(t, secondMinister)

	// Verify both ministers exist and have different IDs
	searchCriteria := &models.SearchCriteria{
		Kind: &models.Kind{
			Major: "Organisation",
			Minor: "cabinetMinister",
		},
		Name: "Duplicate Minister",
	}

	results, err := client.SearchEntities(searchCriteria)
	assert.NoError(t, err)
	results = utils.FilterByExactName(results, "Duplicate Minister")
	assert.Len(t, results, 2, "Should find two ministers with this name")
	assert.NotEqual(t, results[0].ID, results[1].ID, "Ministers should have different IDs")

}
