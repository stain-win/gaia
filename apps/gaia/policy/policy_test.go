package policy

import (
	"os"
	"testing"

	bolt "go.etcd.io/bbolt"
)

func setupTestDB(t *testing.T) (*bolt.DB, func()) {
	tmpfile, err := os.CreateTemp("", "policy-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	db, err := bolt.Open(tmpfile.Name(), 0600, nil)
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(tmpfile.Name())
	}

	return db, cleanup
}

func TestNewStore(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	if store == nil {
		t.Fatal("Store is nil")
	}

	// Verify bucket was created
	err = db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(policiesBucket)
		if bucket == nil {
			t.Error("Policies bucket was not created")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSetAndGetPolicy(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store, _ := NewStore(db)

	policy := Policy{
		ClientName: "test-client",
		Rules: []PolicyRule{
			{
				Path:         "common/*",
				Capabilities: []Capability{CapabilityRead},
				Description:  "Read common",
			},
		},
	}

	// Set policy
	err := store.SetPolicy(policy)
	if err != nil {
		t.Fatalf("Failed to set policy: %v", err)
	}

	// Get policy
	retrieved, err := store.GetPolicy("test-client")
	if err != nil {
		t.Fatalf("Failed to get policy: %v", err)
	}

	if retrieved.ClientName != policy.ClientName {
		t.Errorf("Expected client name %s, got %s", policy.ClientName, retrieved.ClientName)
	}

	if len(retrieved.Rules) != len(policy.Rules) {
		t.Errorf("Expected %d rules, got %d", len(policy.Rules), len(retrieved.Rules))
	}
}

func TestGetPolicyNotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store, _ := NewStore(db)

	_, err := store.GetPolicy("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent policy, got nil")
	}
}

func TestDeletePolicy(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store, _ := NewStore(db)

	policy := Policy{
		ClientName: "test-client",
		Rules: []PolicyRule{
			{Path: "test/*", Capabilities: []Capability{CapabilityRead}},
		},
	}

	store.SetPolicy(policy)

	// Delete
	err := store.DeletePolicy("test-client")
	if err != nil {
		t.Fatalf("Failed to delete policy: %v", err)
	}

	// Verify deleted
	_, err = store.GetPolicy("test-client")
	if err == nil {
		t.Error("Policy still exists after deletion")
	}
}

func TestListPolicies(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store, _ := NewStore(db)

	// Add multiple policies
	policies := []Policy{
		{
			ClientName: "client1",
			Rules:      []PolicyRule{{Path: "client1/*", Capabilities: []Capability{CapabilityRead}}},
		},
		{
			ClientName: "client2",
			Rules:      []PolicyRule{{Path: "client2/*", Capabilities: []Capability{CapabilityWrite}}},
		},
	}

	for _, p := range policies {
		store.SetPolicy(p)
	}

	// List all
	retrieved, err := store.ListPolicies()
	if err != nil {
		t.Fatalf("Failed to list policies: %v", err)
	}

	if len(retrieved) != len(policies) {
		t.Errorf("Expected %d policies, got %d", len(policies), len(retrieved))
	}
}

func TestCheckPermission(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store, _ := NewStore(db)

	policy := Policy{
		ClientName: "test-client",
		Rules: []PolicyRule{
			{
				Path:         "common/*",
				Capabilities: []Capability{CapabilityRead, CapabilityList},
			},
			{
				Path:         "test-client/*",
				Capabilities: []Capability{CapabilityRead, CapabilityWrite, CapabilityDelete},
			},
		},
	}
	store.SetPolicy(policy)

	tests := []struct {
		name       string
		clientName string
		path       string
		capability Capability
		wantErr    bool
	}{
		{
			name:       "allowed read on common",
			clientName: "test-client",
			path:       "common/production/db_url",
			capability: CapabilityRead,
			wantErr:    false,
		},
		{
			name:       "denied write on common",
			clientName: "test-client",
			path:       "common/production/db_url",
			capability: CapabilityWrite,
			wantErr:    true,
		},
		{
			name:       "allowed write on own namespace",
			clientName: "test-client",
			path:       "test-client/production/secret",
			capability: CapabilityWrite,
			wantErr:    false,
		},
		{
			name:       "denied access to other client",
			clientName: "test-client",
			path:       "other-client/production/secret",
			capability: CapabilityRead,
			wantErr:    true,
		},
		{
			name:       "gaia client has all permissions",
			clientName: "gaia",
			path:       "any/path/anywhere",
			capability: CapabilityWrite,
			wantErr:    false,
		},
		{
			name:       "no policy = deny",
			clientName: "unknown-client",
			path:       "common/test",
			capability: CapabilityRead,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.CheckPermission(tt.clientName, tt.path, tt.capability)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPermission() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPathMatches(t *testing.T) {
	tests := []struct {
		requestedPath string
		policyPath    string
		want          bool
	}{
		{"common/production/db", "common/*", true},
		{"common/production/db", "common/production/*", true},
		{"common/production/db", "common/production/db", true},
		{"other/path", "common/*", false},
		{"commontest", "common/*", false},
		{"myapp/prod/secret", "myapp/*", true},
		{"myapp/prod/secret", "myapp/prod/*", true},
		{"myapp/prod/secret", "other/*", false},
	}

	for _, tt := range tests {
		t.Run(tt.requestedPath+"_"+tt.policyPath, func(t *testing.T) {
			got := pathMatches(tt.requestedPath, tt.policyPath)
			if got != tt.want {
				t.Errorf("pathMatches(%q, %q) = %v, want %v", tt.requestedPath, tt.policyPath, got, tt.want)
			}
		})
	}
}

func TestCreateDefaultPolicy(t *testing.T) {
	policy := CreateDefaultPolicy("test-client")

	if policy.ClientName != "test-client" {
		t.Errorf("Expected client name test-client, got %s", policy.ClientName)
	}

	if len(policy.Rules) == 0 {
		t.Error("Default policy has no rules")
	}

	// Should have common read access
	hasCommonRead := false
	hasOwnFullAccess := false

	for _, rule := range policy.Rules {
		if rule.Path == "common/*" {
			hasCommonRead = true
			// Check for read capability
			for _, cap := range rule.Capabilities {
				if cap != CapabilityRead && cap != CapabilityList {
					t.Errorf("Common should only have read/list, got %s", cap)
				}
			}
		}
		if rule.Path == "test-client/*" {
			hasOwnFullAccess = true
			// Should have all capabilities
			if len(rule.Capabilities) < 3 {
				t.Error("Own namespace should have full access")
			}
		}
	}

	if !hasCommonRead {
		t.Error("Default policy missing common read access")
	}
	if !hasOwnFullAccess {
		t.Error("Default policy missing own namespace full access")
	}
}

func TestValidatePolicy(t *testing.T) {
	tests := []struct {
		name    string
		policy  Policy
		wantErr bool
	}{
		{
			name: "valid policy",
			policy: Policy{
				ClientName: "test",
				Rules: []PolicyRule{
					{Path: "common/*", Capabilities: []Capability{CapabilityRead}},
				},
			},
			wantErr: false,
		},
		{
			name: "empty client name",
			policy: Policy{
				ClientName: "",
				Rules:      []PolicyRule{{Path: "test/*", Capabilities: []Capability{CapabilityRead}}},
			},
			wantErr: true,
		},
		{
			name: "no rules",
			policy: Policy{
				ClientName: "test",
				Rules:      []PolicyRule{},
			},
			wantErr: true,
		},
		{
			name: "empty path",
			policy: Policy{
				ClientName: "test",
				Rules:      []PolicyRule{{Path: "", Capabilities: []Capability{CapabilityRead}}},
			},
			wantErr: true,
		},
		{
			name: "no capabilities",
			policy: Policy{
				ClientName: "test",
				Rules:      []PolicyRule{{Path: "test/*", Capabilities: []Capability{}}},
			},
			wantErr: true,
		},
		{
			name: "invalid capability",
			policy: Policy{
				ClientName: "test",
				Rules:      []PolicyRule{{Path: "test/*", Capabilities: []Capability{"invalid"}}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePolicy(tt.policy)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePolicy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
