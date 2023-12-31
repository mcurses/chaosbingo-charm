package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
)

var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	statusMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"}).
				Render
)

type Prompt struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Id          int    `json:"id"` // Add this line
}

// Define an adapter type that wraps Prompt and satisfies list.Item
type PromptItem struct {
	Prompt
}

func (pi PromptItem) Title() string       { return pi.Prompt.Title }
func (pi PromptItem) Description() string { return pi.Prompt.Description }
func (pi PromptItem) FilterValue() string { return pi.Prompt.Title } // or whatever makes sense for filtering

type listKeyMap struct {
	toggleSpinner    key.Binding
	toggleTitleBar   key.Binding
	toggleStatusBar  key.Binding
	togglePagination key.Binding
	toggleHelpMenu   key.Binding
	insertItem       key.Binding
	deleteItem       key.Binding
}

func newListKeyMap() *listKeyMap {
	return &listKeyMap{
		insertItem: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add Prompt"),
		),
		deleteItem: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "delete Prompt"),
		),
		toggleSpinner: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "toggle spinner"),
		),
		toggleTitleBar: key.NewBinding(
			key.WithKeys("T"),
			key.WithHelp("T", "toggle title"),
		),
		toggleStatusBar: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "toggle status"),
		),
		togglePagination: key.NewBinding(
			key.WithKeys("P"),
			key.WithHelp("P", "toggle pagination"),
		),
		toggleHelpMenu: key.NewBinding(
			key.WithKeys("H"),
			key.WithHelp("H", "toggle help"),
		),
	}
}

type model struct {
	list          list.Model
	itemGenerator *randomItemGenerator
	keys          *listKeyMap
	delegateKeys  *delegateKeyMap
}

func newModel() model {
	var (
		itemGenerator randomItemGenerator
		delegateKeys  = newDelegateKeyMap()
		listKeys      = newListKeyMap()
	)

	items := make([]list.Item, 0) // Start with an empty list

	// // Make initial list of items
	// const numItems = 24
	// items := make([]list.Item, numItems)
	// for i := 0; i < numItems; i++ {
	// 	items[i] = itemGenerator.next()
	// }

	// Setup list
	delegate := newItemDelegate(delegateKeys)
	promptPool := list.New(items, delegate, 0, 0)
	promptPool.Title = "Prompt Pool"
	promptPool.Styles.Title = titleStyle
	promptPool.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			listKeys.toggleSpinner,
			listKeys.insertItem,
			listKeys.toggleTitleBar,
			listKeys.toggleStatusBar,
			listKeys.togglePagination,
			listKeys.toggleHelpMenu,
		}
	}

	return model{
		list:          promptPool,
		keys:          listKeys,
		delegateKeys:  delegateKeys,
		itemGenerator: &itemGenerator,
	}
}

func (m model) Init() tea.Cmd {
	return tea.EnterAltScreen
}

var newPromptsChan = make(chan []PromptItem)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	select {
	case newPromptItems := <-newPromptsChan:
		// Handle updating the list with new prompts
		var newListItems []list.Item
		for _, item := range newPromptItems {
			newListItems = append(newListItems, item)
		}
		m.list.SetItems(newListItems)
		return m, nil
	default:
		// Handle other messages (e.g., key presses)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := appStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	case tea.KeyMsg:
		// Don't match any of the keys below if we're actively filtering.
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, m.keys.toggleSpinner):
			cmd := m.list.ToggleSpinner()
			return m, cmd

		case key.Matches(msg, m.keys.toggleTitleBar):
			v := !m.list.ShowTitle()
			m.list.SetShowTitle(v)
			m.list.SetShowFilter(v)
			m.list.SetFilteringEnabled(v)
			return m, nil

		case key.Matches(msg, m.keys.toggleStatusBar):
			m.list.SetShowStatusBar(!m.list.ShowStatusBar())
			return m, nil

		case key.Matches(msg, m.keys.togglePagination):
			m.list.SetShowPagination(!m.list.ShowPagination())
			return m, nil

		case key.Matches(msg, m.keys.toggleHelpMenu):
			m.list.SetShowHelp(!m.list.ShowHelp())
			return m, nil

		case key.Matches(msg, m.keys.deleteItem):
			if selected, ok := m.list.SelectedItem().(PromptItem); ok {
				// Send delete request to server
				err := deletePrompt(selected.Id) // You'll need to ensure PromptItem includes Id
				if err != nil {
					log.Println("Error deleting prompt:", err)
				} else {
					// Handle successful deletion, UI will update from WebSocket broadcast
				}
			}
			return m, nil

		case key.Matches(msg, m.keys.insertItem):
			// Here, you would capture or define the title and description for the new prompt
			// For now, let's use placeholder values
			title := "New Prompt Title"
			description := "New Prompt Description"

			// Send the insert request to the backend
			err := insertPrompt(title, description)
			if err != nil {
				log.Println("Error inserting new prompt:", err)
			} else {
				// Optionally display a status message or handle the successful insert
			}

			// Return the model as is because the actual update will be handled via WebSocket
			return m, nil
		}
	}

	// This will also call our delegate's update function.
	newListModel, cmd := m.list.Update(msg)
	m.list = newListModel
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	return appStyle.Render(m.list.View())
}

var wsConn *websocket.Conn

func deletePrompt(id int) error {
	// Construct the URL with the prompt ID
	url := fmt.Sprintf("http://127.0.0.1:8000/prompts/delete/%d", id)

	// Create a new HTTP request for deletion
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	// Send the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() // Always close the response body

	// Check response status code for success or failure
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server responded with status code %d", resp.StatusCode)
	}

	return nil
}
func connectWebSocket() {
	// Adjust the URL to your FastAPI WebSocket endpoint
	u := "ws://127.0.0.1:8000/ws"

	var err error
	wsConn, _, err = websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		log.Fatalf("Error connecting to WebSocket: %v", err)
	}
}

type newPromptsMsg []PromptItem

func listenAndUpdateList() {
	for {
		_, message, err := wsConn.ReadMessage()
		if err != nil {
			log.Println("Error reading from WebSocket:", err)
			break
		}

		var prompts []Prompt
		if err := json.Unmarshal(message, &prompts); err != nil {
			log.Println("Error decoding message:", err)
			continue
		}

		var newPromptItems []PromptItem
		for _, p := range prompts {
			newPromptItems = append(newPromptItems, PromptItem{Prompt: p})
		}

		// Send the new prompts to the channel
		newPromptsChan <- newPromptItems
	}
}

func insertPrompt(title, description string) error {
	newPrompt := Prompt{Title: title, Description: description}
	jsonData, err := json.Marshal(newPrompt)
	if err != nil {
		return err
	}
	log.Println("json", string(jsonData), string(newPrompt.Title), string(newPrompt.Description))

	_, err = http.Post("http://127.0.0.1:8000/prompts/", "application/json", bytes.NewBuffer(jsonData))
	return err
}

func fetchInitialPrompts() ([]Prompt, error) {
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

func main() {

	// Open or create the file
	file, err := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal("Error opening debug.log:", err)
	}
	defer file.Close()

	// Set the output of the log package to the file
	log.SetOutput(file)

	rand.Seed(time.Now().UTC().UnixNano())

	m := newModel() // Initialize model

	// Assuming fetchInitialPrompts returns a slice of Prompt
	initialPrompts, err := fetchInitialPrompts()
	if err != nil {
		log.Fatalf("Error fetching initial prompts: %v", err)
	}

	// Update the model's list with fetched prompts
	var newItems []list.Item
	for _, p := range initialPrompts {
		promptItem := PromptItem{Prompt: p}     // Creating a PromptItem from each Prompt
		newItems = append(newItems, promptItem) // Appending the PromptItem to newItems
	}
	m.list.SetItems(newItems)

	connectWebSocket()   // Connect to WebSocket
	defer wsConn.Close() // Close the connection when done

	// Run the program in a separate goroutine
	go func() {
		if _, err := tea.NewProgram(m).Run(); err != nil {
			fmt.Println("Error running program:", err)
			os.Exit(1)
		}
	}()

	// Listen for WebSocket messages and update list
	listenAndUpdateList()

	select {}
}
