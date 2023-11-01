package main

import (
	"context"
	"fmt"
	"log"
	"os"

	firebase "firebase.google.com/go"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gempir/go-twitch-irc/v4"
)

type model struct {
	client   *twitch.Client
	choices  []string                   // Twitch channels to join
	cursor   int                        // which channel our cursor is pointing at
	selected map[int]struct{}           // which channels are selected
	messages map[string][]string        // messages by channel
	msgChan  chan twitch.PrivateMessage // a channel to receive messages from Twitch
}

func initialModel() model {
	client := twitch.NewAnonymousClient()
	msgChan := make(chan twitch.PrivateMessage, 100)
	return model{
		client:   client,
		choices:  []string{"kingsleague", "riversgg", "thegrefg", "elspreen", "aroyitt"},
		selected: make(map[int]struct{}),
		messages: make(map[string][]string),
		msgChan:  msgChan,
	}
}

func (m model) Init() tea.Cmd {
	for _, channel := range m.choices {
		m.client.Join(channel)
	}

	m.client.OnPrivateMessage(func(message twitch.PrivateMessage) {
		m.msgChan <- message
	})

	go m.client.Connect()

	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.client.Disconnect()
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter", " ":
			_, ok := m.selected[m.cursor]
			if ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = struct{}{}
			}
		}
	case twitch.PrivateMessage:
		// Handle the twitch message
		channel := msg.Channel
		m.messages[channel] = append(m.messages[channel], msg.Message)

		// Only keep the last 10 messages per channel
		if len(m.messages[channel]) > 10 {
			m.messages[channel] = m.messages[channel][1:]
		}
	}

	return m, nil
}

func (m model) View() string {
	s := "Select Channel\n\n"

	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		checked := " "
		if _, ok := m.selected[i]; ok {
			checked = "x"
		}
		s += fmt.Sprintf("%s [%s] %s\n", cursor, checked, choice)
	}

	s += "\nLast messages:\n"
	for channel, messages := range m.messages {
		// Check if the channel is selected
		if _, ok := m.selected[channelIndex(channel)]; ok {
			border := "+---------------------------------+\n" // adjust as needed
			s += border
			s += fmt.Sprintf("| %s:\n", channel)
			for _, message := range messages {
				s += "| " + message + "\n"
			}
			s += border + "\n"
		}
	}

	s += "\nPress q to quit.\n"
	return s
}

func channelIndex(channel string) int {
	for i, ch := range initialModel().choices {
		if ch == channel {
			return i
		}
	}
	return -1
}

var p *tea.Program

func main() {
	_, err := firebase.NewApp(context.Background(), nil)
	if err != nil {
		log.Fatalf("error initializing app: %v\n", err)
	}

	m := initialModel()
	p = tea.NewProgram(m)

	// Run a goroutine to continuously send Twitch messages to the program.
	go func() {
		for message := range m.msgChan {
			p.Send(message)
		}
	}()

	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
