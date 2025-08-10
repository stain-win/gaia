package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type InputElement interface {
	Blink() tea.Msg
	Blur() tea.Msg
	Focus() tea.Cmd
	SetValue(string)
	Value() string
	Update(tea.Msg) (InputElement, tea.Cmd)
	View() string
}

type singleLineInput struct {
	textinput textinput.Model
}

func newSingleLineInput() *singleLineInput {
	s := singleLineInput{}

	model := textinput.New()
	model.Placeholder = "Enter text here"
	model.Focus()
	s.textinput = model
	return &s
}

func (s *singleLineInput) Init() tea.Cmd {
	return nil
}

func (s *singleLineInput) Blink() tea.Msg {
	return textinput.Blink()
}

func (s *singleLineInput) SetValue(value string) {
	s.textinput.SetValue(value)
}

func (s *singleLineInput) Value() string {
	return s.textinput.Value()
}

func (s *singleLineInput) Blur() tea.Msg {
	return s.textinput.Blur
}

func (s *singleLineInput) Focus() tea.Cmd {
	return s.textinput.Focus()
}

func (s *singleLineInput) View() string {
	return s.textinput.View()
}

func (s *singleLineInput) Update(msg tea.Msg) (InputElement, tea.Cmd) {
	var cmd tea.Cmd
	s.textinput, cmd = s.textinput.Update(msg)
	return s, cmd
}
