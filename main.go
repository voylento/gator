package main

import (
	"fmt"
	"github.com/voylento/gator/internal/config"
	"os"
)

type State struct {
	config *config.Config
}

type Command struct {
	name		string
	args		[]string
}

type Commands struct {
	callbacks	map[string]func(*State, Command) error
}

func (c *Commands) Run(s *State, cmd Command) error {
	f, ok := c.callbacks[cmd.name]
	if !ok {
		return fmt.Errorf("unknown command: %s", cmd.name)
	}

	err := f(s, cmd)
	if err != nil {
		return fmt.Errorf("%w\n", err)
	}

	return nil
}

func (c *Commands) Register(name string, f func(*State, Command) error) {
	c.callbacks[name] = f
}

func InitializeApp() (*Commands, *State) {
	cmds := &Commands{
		callbacks: 	make(map[string]func(*State, Command) error),
	}

	cmds.Register("login", handleLogin)

	cfg, err := config.ReadConfig()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	s := &State{
		config: cfg,
	}

	return cmds, s
}


func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: gator <command> [arguments]")
		os.Exit(1)
	}

	commands, state := InitializeApp()

	command := os.Args[1]

	switch command {
	case "login":
		cmd := Command {
			name: "login",
			args: os.Args[2:],
		}
		err := commands.Run(state, cmd)
		if err != nil {
			fmt.Printf("%v", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}

func handleLogin(s *State, cmd Command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("Usage: gator login <username>\n")
	}

	s.config.SetUser(cmd.args[0])

	return nil
}

