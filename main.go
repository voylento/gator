package main

import (
  _ "github.com/lib/pq"
	"context"
	"database/sql"
	"fmt"
	"github.com/voylento/gator/internal/config"
	"github.com/voylento/gator/internal/database"
	"github.com/google/uuid"
	"time"
	"os"
)

type State struct {
	config 	*config.Config
	db 			*database.Queries
}

type Command struct {
	name		string
	args		[]string
}

type CommandMap struct {
	callbacks	map[string]func(*State, Command) error
}

func (c *CommandMap) Run(s *State, cmd Command) error {
	f, ok := c.callbacks[cmd.name]
	if !ok {
		return fmt.Errorf("Unknown command: %s\n", c)
	}

	err := f(s, cmd)
	if err != nil {
		return fmt.Errorf("%w\n", err)
		os.Exit(1)
	}

	return nil
}

func (c *CommandMap) Register(name string, f func(*State, Command) error) {
	c.callbacks[name] = f
}

func InitializeApp() (*State, *CommandMap) {
	cmds := &CommandMap{
		callbacks: 	make(map[string]func(*State, Command) error),
	}

	cmds.Register("login", handleLogin)
	cmds.Register("register", handleRegister)
	cmds.Register("reset", handleReset)

	cfg, err := config.ReadConfig()
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	db, err := sql.Open("postgres", cfg.DbUrl)
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
	
	dbQueries := database.New(db)

	s := &State{
		config: cfg,
		db: dbQueries,
	}

	return s, cmds
}

func main() {
	args := os.Args
	var commandArgs []string

	if len(args) < 2 {
		fmt.Println("Usage: gator <command> [arguments]")
		os.Exit(1)
	} else if len(args) > 2 {
		commandArgs = args[2:]
	}

	state, commands := InitializeApp()

	cmd := Command{
		name: args[1],
		args: commandArgs,
	}

	err := commands.Run(state, cmd)
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}

func handleLogin(s *State, cmd Command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("Usage: gator login <username>\n")
	}

	_, err := s.db.GetUser(context.Background(), cmd.args[0])
	if err != nil {
		return err
	}
	
	s.config.SetUser(cmd.args[0])

	return nil
}

func handleRegister(s *State, cmd Command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("Usage: gator register <username>\n")
	}
	
	timeNow := time.Now()
	userParams := database.CreateUserParams{
		ID:					uuid.New(),
		CreatedAt:	timeNow,
		UpdatedAt:	timeNow,
		Name:				cmd.args[0],
	}

	user, err := s.db.CreateUser(context.Background(), userParams)
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	s.config.SetUser(cmd.args[0])

	fmt.Printf("Register %v succeeded\n", cmd.args[0])
	fmt.Printf("User.ID:\t%v\nUser.CreatedAt:\t%v\nUser.UpdatedAt:\t%v\nUser.Name:\t%v\n", user.ID, user.CreatedAt, user.UpdatedAt, user.Name)

	return nil
}

func handleReset(s *State, cmd Command) error {
	if len(cmd.args) > 0 {
		return fmt.Errorf("Usage: gator reset\n")
	}

	err := s.db.DeleteAllUsers(context.Background())
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	fmt.Println("All users deleted from database gator")

	return nil
}

