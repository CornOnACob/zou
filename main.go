package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-ldap/ldap/v3"
	"github.com/joho/godotenv"
	"golang.org/x/term"
)

type ArrayStack [][]string
type Stack []int

func (s *ArrayStack) PushArray(element []string) {
	*s = append(*s, element)
}

func (s *Stack) Push(v int) {
	*s = append(*s, v)
}

func (s *ArrayStack) PopArray() []string {
	if len(*s) == 0 {
		return nil
	}
	index := len(*s) - 1 // Get the index of the top element
	element := (*s)[index]
	*s = (*s)[:index] // Remove the element by slicing
	return element
}

func (s *Stack) Pop() (int, bool) {
	if s.IsEmpty() {
		return 0, false
	} else {
		index := len(*s) - 1
		element := (*s)[index]
		*s = (*s)[:index] // Remove last element
		return element, true
	}
}

func (s *Stack) IsEmpty() bool {
	return len(*s) == 0
}

type model struct {
	ous         [][]string // Items in the picker
	choices     []string
	infoText    string
	cursor      int // Which choice is currently focused
	selected    int // Which choice is selected (enter key)
	depth       int // How deep are we in the OU hierarchy
	ouStack     ArrayStack
	cursorStack Stack
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {

		case "up", "k":
			m.infoText = ""
			if m.cursor > 0 {
				m.cursor--
			} else {
				m.cursor = len(m.choices) - 1
			}

		case "down", "j":
			m.infoText = ""
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}

		case "left":
			m.infoText = ""
			if m.depth > 1 {
				m.choices = m.ouStack.PopArray()
				element, ok := m.cursorStack.Pop()
				if ok {
					m.cursor = element
				}
				m.depth--
			}

		case "right":
			m.infoText = ""
			var newChoices []string
			for i := 0; i < len(m.ous); i++ {
				mergedString := strings.Join(m.ous[i], ",")
				if strings.Contains(mergedString, m.choices[m.cursor]) && len(m.ous[i]) == m.depth+3 {
					newChoices = append(newChoices, mergedString)
				}
			}
			if len(newChoices) > 0 {
				m.ouStack.PushArray(m.choices)
				m.cursorStack.Push(m.cursor)
				m.choices = newChoices
				m.cursor = 0
				m.depth++
			} else {
				m.infoText = "THE SELECTED OU IS EMPTY!"
			}

		case "enter":
			m.selected = m.cursor
			return m, tea.Quit

		case "q", "ctrl+c":
			return m, tea.Quit
		}

	}
	return m, nil
}

func (m model) View() string {
	s := "Choose an OU:\n\n"
	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = fmt.Sprintf("> \x1b[30;47m%s\x1b[0m", choice)
		} else {
			cursor = fmt.Sprintf("  %s", choice)
		}
		s += cursor + "\n"
	}
	if m.infoText != "" {
		s += "\n\x1b[33m" + m.infoText + "\x1b[0m\n"
	}
	s += "\nPress q to quit.\n"
	return s
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	ldapServer := os.Getenv("LDAP_SERVER")
	baseDN := os.Getenv("BASE_DN")

	l, err := ldap.Dial("tcp", ldapServer)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	fmt.Print("Enter username: ")
	var username string
	_, err = fmt.Scanln(&username)
	if err != nil {
		log.Fatalf("Failed to read username: %v", err)
	}

	fmt.Print("Enter password: ")
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("Failed to read password: %v", err)
	}
	password := string(passwordBytes)
	fmt.Println()

	err = l.Bind("cn="+username+",ou=Administrators,dc=obm,dc=local", password)
	if err != nil {
		log.Fatal(err)
	}

	searchRequest := ldap.NewSearchRequest(
		baseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=organizationalUnit)", // The filter to apply
		[]string{"dn"},                     // Attributes to retrieve ("dn" gets the distinguished name)
		nil,
	)

	sr, err := l.Search(searchRequest)
	if err != nil {
		log.Fatal(err)
	}

	var entries []string

	for _, entry := range sr.Entries {
		res := entry.DN[3:]
		entries = append(entries, res)
	}

	sort.Strings(entries)

	var cliChoices []string
	var allOUs [][]string

	for _, entry := range entries {
		subStrings := strings.Split(entry, ",")
		allOUs = append(allOUs, subStrings)
		if len(subStrings) <= 3 {
			cliChoices = append(cliChoices, entry)
		}
	}

	initialModel := model{
		ous:      allOUs,
		choices:  cliChoices,
		depth:    1,
		selected: -1,
	}

	p := tea.NewProgram(initialModel)

	finalModel, err := p.Run()
	if err != nil {
		fmt.Printf("Alas, there's been an error: %v\n", err)
		os.Exit(1)
	}

	fm := finalModel.(model)
	if fm.selected >= 0 {
		fmt.Printf("\nYOU CHOSE:\nOU=%s\n\n", fm.choices[fm.selected])
	}
}
