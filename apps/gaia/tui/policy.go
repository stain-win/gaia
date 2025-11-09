package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
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

// policyFormModel represents the form for creating/editing policies
type policyFormModel struct {
	clientName  string
	rules       []policyRuleForm
	currentRule int
	textarea    textarea.Model
	editing     bool
	message     string
}

type policyRuleForm struct {
	path         string
	capabilities []string
	description  string
}

func newPolicyFormModel(clientName string) *policyFormModel {
	ta := textarea.New()
	ta.Placeholder = "Enter path pattern (e.g., common/*, myapp/production/*)"
	ta.CharLimit = 200
	ta.SetHeight(3)
	ta.SetWidth(60)
	ta.Focus()

	return &policyFormModel{
		clientName:  clientName,
		rules:       []policyRuleForm{},
		currentRule: 0,
		textarea:    ta,
		editing:     false,
	}
}

func (m *policyFormModel) addRule(rule policyRuleForm) {
	m.rules = append(m.rules, rule)
}

func (m *policyFormModel) removeRule(index int) {
	if index >= 0 && index < len(m.rules) {
		m.rules = append(m.rules[:index], m.rules[index+1:]...)
	}
}

func (m *policyFormModel) toPolicy() policy.Policy {
	rules := make([]policy.PolicyRule, len(m.rules))
	for i, r := range m.rules {
		caps := make([]policy.Capability, len(r.capabilities))
		for j, c := range r.capabilities {
			caps[j] = policy.Capability(c)
		}
		rules[i] = policy.PolicyRule{
			Path:         r.path,
			Capabilities: caps,
			Description:  r.description,
		}
	}

	return policy.Policy{
		ClientName: m.clientName,
		Rules:      rules,
	}
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
