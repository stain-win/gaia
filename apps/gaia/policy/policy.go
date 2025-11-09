// Package policy provides policy-based authorization for Gaia.
// It defines and enforces access control policies for clients based on
// their certificate Common Name and requested operations.
package policy

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	bolt "go.etcd.io/bbolt"
)

// Capability represents an action that can be performed on a resource.
type Capability string

const (
	CapabilityRead   Capability = "read"
	CapabilityWrite  Capability = "write"
	CapabilityDelete Capability = "delete"
	CapabilityList   Capability = "list"
)

// Policy defines access control rules for a client.
type Policy struct {
	ClientName string       `json:"client_name"`
	Rules      []PolicyRule `json:"rules"`
}

// PolicyRule defines what a client can do on a specific path pattern.
type PolicyRule struct {
	Path         string       `json:"path"`         // Path pattern (e.g., "common/*", "myapp/prod/*")
	Capabilities []Capability `json:"capabilities"` // Allowed actions: read, write, delete, list
	Description  string       `json:"description"`  // Optional description
}

var (
	policiesBucket = []byte("policies")
)

// Store manages policy storage and retrieval.
type Store struct {
	db *bolt.DB
}

// NewStore creates a new policy store.
func NewStore(db *bolt.DB) (*Store, error) {
	// Ensure bucket exists
	err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(policiesBucket)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create policies bucket: %w", err)
	}

	return &Store{db: db}, nil
}

// SetPolicy stores a policy for a client.
func (s *Store) SetPolicy(policy Policy) error {
	if policy.ClientName == "" {
		return fmt.Errorf("client name cannot be empty")
	}

	data, err := json.Marshal(policy)
	if err != nil {
		return fmt.Errorf("failed to marshal policy: %w", err)
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(policiesBucket)
		return bucket.Put([]byte(policy.ClientName), data)
	})
}

// GetPolicy retrieves a policy for a client.
func (s *Store) GetPolicy(clientName string) (*Policy, error) {
	var policy Policy

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(policiesBucket)
		data := bucket.Get([]byte(clientName))
		if data == nil {
			return fmt.Errorf("policy not found for client: %s", clientName)
		}
		return json.Unmarshal(data, &policy)
	})

	if err != nil {
		return nil, err
	}
	return &policy, nil
}

// DeletePolicy removes a policy for a client.
func (s *Store) DeletePolicy(clientName string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(policiesBucket)
		return bucket.Delete([]byte(clientName))
	})
}

// ListPolicies returns all policies.
func (s *Store) ListPolicies() ([]Policy, error) {
	var policies []Policy

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(policiesBucket)
		return bucket.ForEach(func(k, v []byte) error {
			var policy Policy
			if err := json.Unmarshal(v, &policy); err != nil {
				return err
			}
			policies = append(policies, policy)
			return nil
		})
	})

	return policies, err
}

// CheckPermission verifies if a client has permission to perform an action on a path.
func (s *Store) CheckPermission(clientName, path string, capability Capability) error {
	// Special case: "gaia" client (admin) has all permissions
	if clientName == "gaia" {
		return nil
	}

	policy, err := s.GetPolicy(clientName)
	if err != nil {
		// If no policy exists, deny by default
		return fmt.Errorf("no policy found for client %s: access denied", clientName)
	}

	// Check each rule
	for _, rule := range policy.Rules {
		if pathMatches(path, rule.Path) {
			// Check if capability is allowed
			for _, cap := range rule.Capabilities {
				if cap == capability {
					return nil // Permission granted
				}
			}
		}
	}

	return fmt.Errorf("permission denied: client %s cannot %s %s", clientName, capability, path)
}

// pathMatches checks if a requested path matches a policy path pattern.
// Supports wildcards (*) and exact matches.
func pathMatches(requestedPath, policyPath string) bool {
	// Exact match
	if requestedPath == policyPath {
		return true
	}

	// Wildcard matching
	if strings.HasSuffix(policyPath, "/*") {
		prefix := strings.TrimSuffix(policyPath, "/*")
		return strings.HasPrefix(requestedPath, prefix+"/")
	}

	// Single wildcard at the end (e.g., "common/*")
	if strings.HasSuffix(policyPath, "*") {
		prefix := strings.TrimSuffix(policyPath, "*")
		return strings.HasPrefix(requestedPath, prefix)
	}

	// Use filepath.Match for more complex patterns
	matched, _ := filepath.Match(policyPath, requestedPath)
	return matched
}

// CreateDefaultPolicy creates a default policy for a client.
// The default policy allows full access to the client's own namespace.
func CreateDefaultPolicy(clientName string) Policy {
	return Policy{
		ClientName: clientName,
		Rules: []PolicyRule{
			{
				Path:         "common/*",
				Capabilities: []Capability{CapabilityRead, CapabilityList},
				Description:  "Read access to common secrets",
			},
			{
				Path:         clientName + "/*",
				Capabilities: []Capability{CapabilityRead, CapabilityWrite, CapabilityDelete, CapabilityList},
				Description:  "Full access to own namespace",
			},
		},
	}
}

// ValidatePolicy checks if a policy is valid.
func ValidatePolicy(policy Policy) error {
	if policy.ClientName == "" {
		return fmt.Errorf("client name is required")
	}

	if len(policy.Rules) == 0 {
		return fmt.Errorf("at least one rule is required")
	}

	for i, rule := range policy.Rules {
		if rule.Path == "" {
			return fmt.Errorf("rule %d: path is required", i)
		}
		if len(rule.Capabilities) == 0 {
			return fmt.Errorf("rule %d: at least one capability is required", i)
		}
		for _, cap := range rule.Capabilities {
			if !isValidCapability(cap) {
				return fmt.Errorf("rule %d: invalid capability %s", i, cap)
			}
		}
	}

	return nil
}

func isValidCapability(cap Capability) bool {
	switch cap {
	case CapabilityRead, CapabilityWrite, CapabilityDelete, CapabilityList:
		return true
	default:
		return false
	}
}
