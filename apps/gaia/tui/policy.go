package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/stain-win/gaia/apps/gaia/policy"
)

// policyListItem represents a policy in the list view
type policyListItem struct {
	clientName string
	ruleCount  int
	summary    string
}

func (i policyListItem) Title() string { return i.clientName }
func (i policyListItem) Description() string {
	return fmt.Sprintf("%d rules • %s", i.ruleCount, i.summary)
}
func (i policyListItem) FilterValue() string { return i.clientName }

// policyDetailModel represents the detailed view of a single policy
type policyDetailModel struct {
	policy   policy.Policy
	viewport viewport.Model
	ready    bool
}

func newPolicyDetailModel(pol policy.Policy) *policyDetailModel {
	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1)

	return &policyDetailModel{
		policy:   pol,
		viewport: vp,
		ready:    false,
	}
}

func (m *policyDetailModel) setSize(width, height int) {
	m.viewport.Width = width - 4
	m.viewport.Height = height - 10
	m.ready = true
	m.updateContent()
}

func (m *policyDetailModel) updateContent() {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF8C00")).
		MarginBottom(1)

	ruleStyle := lipgloss.NewStyle().
		MarginLeft(2).
		MarginBottom(1)

	pathStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00CED1")).
		Bold(true)

	capStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#90EE90"))

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Italic(true)

	b.WriteString(titleStyle.Render(fmt.Sprintf("Policy for: %s", m.policy.ClientName)))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Total Rules: %d\n\n", len(m.policy.Rules)))

	for i, rule := range m.policy.Rules {
		b.WriteString(ruleStyle.Render(fmt.Sprintf("Rule %d:", i+1)))
		b.WriteString("\n")
		b.WriteString(ruleStyle.Render(fmt.Sprintf("  Path: %s", pathStyle.Render(rule.Path))))
		b.WriteString("\n")

		caps := make([]string, len(rule.Capabilities))
		for j, cap := range rule.Capabilities {
			caps[j] = capStyle.Render(string(cap))
		}
		b.WriteString(ruleStyle.Render(fmt.Sprintf("  Capabilities: %s", strings.Join(caps, ", "))))
		b.WriteString("\n")

		if rule.Description != "" {
			b.WriteString(ruleStyle.Render(fmt.Sprintf("  Description: %s", descStyle.Render(rule.Description))))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	m.viewport.SetContent(b.String())
}

// renderPolicyList renders the list of policies
func renderPolicyList(policies []policyListItem, width, height int) list.Model {
	items := make([]list.Item, len(policies))
	for i, p := range policies {
		items[i] = p
	}

	l := list.New(items, list.NewDefaultDelegate(), width-4, height-10)
	l.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF8C00")).
		Render("Authorization Policies")

	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	if len(policies) == 0 {
		l.SetShowHelp(false)
		l.SetShowTitle(false)
	}

	return l
}

// buildAccessSummary creates a short summary of what a policy grants access to
func buildAccessSummary(pol policy.Policy) string {
	if len(pol.Rules) == 0 {
		return "No access"
	}

	hasCommon := false
	hasOwn := false
	crossAccess := 0

	for _, rule := range pol.Rules {
		if strings.HasPrefix(rule.Path, "common/") || rule.Path == "common/*" {
			hasCommon = true
		} else if strings.HasPrefix(rule.Path, pol.ClientName+"/") || rule.Path == pol.ClientName+"/*" {
			hasOwn = true
		} else {
			crossAccess++
		}
	}

	var parts []string
	if hasOwn {
		parts = append(parts, "own namespace")
	}
	if hasCommon {
		parts = append(parts, "common")
	}
	if crossAccess > 0 {
		parts = append(parts, fmt.Sprintf("+%d cross-namespace", crossAccess))
	}

	if len(parts) == 0 {
		return "custom access"
	}

	return strings.Join(parts, ", ")
}

// Helper styles for policy views
var (
	policyTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FF8C00")).
				MarginBottom(1)

	policyPathStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00CED1")).
			Bold(true)

	policyCapStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#90EE90"))

	policyDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Italic(true)

	policyErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF0000")).
				Bold(true)

	policySuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00FF00")).
				Bold(true)
)

// Policy Editor with huh forms

// policyEditorModel handles creating and editing policies
type policyEditorModel struct {
	form             *huh.Form
	clientName       string
	existingPolicy   *policy.Policy
	step             int // 0=client selection, 1=template/manual, 2=rule building
	rules            []policy.PolicyRule
	selectedTemplate string
	message          string
}

func newPolicyEditorModel(clientName string, existingPolicy *policy.Policy) *policyEditorModel {
	editor := &policyEditorModel{
		clientName:     clientName,
		existingPolicy: existingPolicy,
		step:           0,
		rules:          []policy.PolicyRule{},
	}

	if existingPolicy != nil {
		editor.rules = existingPolicy.Rules
		editor.step = 2 // Jump to rule editing
	}

	return editor
}

func (m *policyEditorModel) Init() *huh.Form {
	if m.clientName == "" {
		// Step 0: Client selection
		return huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Key("clientName").
					Title("Client Name").
					Description("Name of the client (must match certificate CN)").
					Placeholder("myapp").
					Validate(func(s string) error {
						if s == "" {
							return fmt.Errorf("client name is required")
						}
						if s == "common" || s == "gaia" {
							return fmt.Errorf("'%s' is a reserved name", s)
						}
						return nil
					}),
			),
		)
	}

	// Step 1: Template selection (for new policies)
	if m.existingPolicy == nil && m.step == 0 {
		return huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Key("template").
					Title("Policy Template").
					Description("Start with a template or create from scratch").
					Options(
						huh.NewOption("Default (own namespace + common read)", "default"),
						huh.NewOption("Read-Only Monitoring", "readonly"),
						huh.NewOption("Microservices (cross-namespace access)", "microservices"),
						huh.NewOption("Custom (start from scratch)", "custom"),
					).
					Value(&m.selectedTemplate),
			),
		)
	}

	return nil
}

func (m *policyEditorModel) applyTemplate() {
	switch m.selectedTemplate {
	case "default":
		m.rules = policy.CreateDefaultPolicy(m.clientName).Rules
	case "readonly":
		m.rules = []policy.PolicyRule{
			{
				Path:         "*/production/*",
				Capabilities: []policy.Capability{policy.CapabilityRead, policy.CapabilityList},
				Description:  "Read-only access to all production namespaces",
			},
		}
	case "microservices":
		m.rules = []policy.PolicyRule{
			{
				Path:         "common/*",
				Capabilities: []policy.Capability{policy.CapabilityRead},
				Description:  "Read common secrets",
			},
			{
				Path:         m.clientName + "/*",
				Capabilities: []policy.Capability{policy.CapabilityRead, policy.CapabilityWrite, policy.CapabilityDelete, policy.CapabilityList},
				Description:  "Full access to own namespace",
			},
		}
	case "custom":
		m.rules = []policy.PolicyRule{}
	}
}

func (m *policyEditorModel) buildPolicy() policy.Policy {
	return policy.Policy{
		ClientName: m.clientName,
		Rules:      m.rules,
	}
}

// Rule editor form
func createRuleForm(rule *policy.PolicyRule) *huh.Form {
	if rule == nil {
		rule = &policy.PolicyRule{}
	}

	// Capability checkboxes
	var hasRead, hasWrite, hasDelete, hasList bool
	for _, cap := range rule.Capabilities {
		switch cap {
		case policy.CapabilityRead:
			hasRead = true
		case policy.CapabilityWrite:
			hasWrite = true
		case policy.CapabilityDelete:
			hasDelete = true
		case policy.CapabilityList:
			hasList = true
		}
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("path").
				Title("Path Pattern").
				Description("e.g., common/*, myapp/production/*").
				Placeholder("common/*").
				Value(&rule.Path).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("path is required")
					}
					return nil
				}),
			huh.NewMultiSelect[string]().
				Key("capabilities").
				Title("Capabilities").
				Description("Select allowed actions").
				Options(
					huh.NewOption("Read (get secrets)", "read").Selected(hasRead),
					huh.NewOption("Write (add/update secrets)", "write").Selected(hasWrite),
					huh.NewOption("Delete (remove secrets)", "delete").Selected(hasDelete),
					huh.NewOption("List (list namespaces)", "list").Selected(hasList),
				),
			huh.NewInput().
				Key("description").
				Title("Description (optional)").
				Placeholder("Access description").
				Value(&rule.Description),
		),
	)
}

// Policy delete confirmation model
type policyDeleteModel struct {
	clientName string
	policy     policy.Policy
	confirmed  bool
	form       *huh.Form
}

func newPolicyDeleteModel(clientName string, pol policy.Policy) *policyDeleteModel {
	var confirmed bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Key("confirm").
				Title(fmt.Sprintf("Delete policy for '%s'?", clientName)).
				Description(fmt.Sprintf("This will remove %d rules and deny all access. This action cannot be undone.", len(pol.Rules))).
				Affirmative("Yes, delete").
				Negative("No, cancel").
				Value(&confirmed),
		),
	)

	return &policyDeleteModel{
		clientName: clientName,
		policy:     pol,
		confirmed:  confirmed,
		form:       form,
	}
}

// Policy export functionality
func exportPolicyToFile(pol policy.Policy, format string) error {
	filename := fmt.Sprintf("%s-policy-%s.%s", pol.ClientName, time.Now().Format("20060102-150405"), format)

	var data []byte
	var err error

	if format == "json" {
		data, err = json.MarshalIndent(pol, "", "  ")
	} else {
		// YAML format (simple implementation)
		data = []byte(formatPolicyAsYAML(pol))
	}

	if err != nil {
		return fmt.Errorf("failed to marshal policy: %w", err)
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func formatPolicyAsYAML(pol policy.Policy) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("client_name: %s\n", pol.ClientName))
	b.WriteString("rules:\n")
	for _, rule := range pol.Rules {
		b.WriteString(fmt.Sprintf("  - path: %s\n", rule.Path))
		b.WriteString("    capabilities:\n")
		for _, cap := range rule.Capabilities {
			b.WriteString(fmt.Sprintf("      - %s\n", cap))
		}
		if rule.Description != "" {
			b.WriteString(fmt.Sprintf("    description: %s\n", rule.Description))
		}
	}
	return b.String()
}

// Policy validation feedback
func validatePolicyWithFeedback(pol policy.Policy) (bool, []string) {
	var warnings []string

	err := policy.ValidatePolicy(pol)
	if err != nil {
		return false, []string{err.Error()}
	}

	// Check for common issues
	if len(pol.Rules) == 0 {
		warnings = append(warnings, "⚠️ Policy has no rules - client will have no access")
	}

	// Check for overly permissive rules
	for i, rule := range pol.Rules {
		if rule.Path == "*" || rule.Path == "*/*" {
			warnings = append(warnings, fmt.Sprintf("⚠️ Rule %d: Wildcard path '*' grants access to everything", i+1))
		}

		if len(rule.Capabilities) == 4 {
			warnings = append(warnings, fmt.Sprintf("⚠️ Rule %d: All capabilities granted on %s", i+1, rule.Path))
		}
	}

	// Check for duplicate paths
	seen := make(map[string]bool)
	for i, rule := range pol.Rules {
		if seen[rule.Path] {
			warnings = append(warnings, fmt.Sprintf("⚠️ Rule %d: Duplicate path '%s'", i+1, rule.Path))
		}
		seen[rule.Path] = true
	}

	return true, warnings
}

// Client selector for policy operations
func createClientSelector(clients []string, title string) *huh.Form {
	options := make([]huh.Option[string], len(clients))
	for i, client := range clients {
		options[i] = huh.NewOption(client, client)
	}

	var selected string
	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Key("client").
				Title(title).
				Description("Select a client to manage their policy").
				Options(options...).
				Value(&selected),
		),
	)
}

// Template descriptions for help text
var policyTemplateDescriptions = map[string]string{
	"default": `Default Policy:
- Read-only access to common/* (shared secrets)
- Full access to own namespace (read, write, delete, list)

Best for: Standard application clients`,

	"readonly": `Read-Only Monitoring:
- Read and list access to all production namespaces
- No write or delete permissions

Best for: Monitoring tools, audit systems`,

	"microservices": `Microservices Template:
- Read access to common secrets
- Full access to own namespace
- Add custom cross-service access as needed

Best for: Services that need to access other services' secrets`,

	"custom": `Custom Policy:
- Start with no rules
- Build policy from scratch
- Full control over all permissions

Best for: Special cases and fine-grained control`,
}
