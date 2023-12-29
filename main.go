package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	// "io/ioutil"
	"net/http"
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type Prompt struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
}

type model struct {
	list list.Model
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return docStyle.Render(m.list.View())
}

func fetchPrompts() ([]Prompt, error) {
	resp, err := http.Get("http://127.0.0.1:8000/prompts/")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var prompts []Prompt
	if err := json.NewDecoder(resp.Body).Decode(&prompts); err != nil {
		return nil, err
	}
	return prompts, nil
}

func addPrompt(text string) error {
	prompt := Prompt{Text: text}
	jsonValue, _ := json.Marshal(prompt)
	resp, err := http.Post("http://127.0.0.1:8000/prompts/", "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func deletePrompt(id int) error {
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", fmt.Sprintf("http://127.0.0.1:8000/prompts/%d", id), nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func main() {
	prompts, err := fetchPrompts()
	if err != nil {
		fmt.Println("Error fetching prompts:", err)
		os.Exit(1)
	}

	var items []list.Item
	for _, p := range prompts {
		items = append(items, item{title: fmt.Sprintf("Prompt %d", p.ID), desc: p.Text})
	}

	m := model{list: list.New(items, list.NewDefaultDelegate(), 0, 0)}
	m.list.Title = "My Fave Things"

	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

