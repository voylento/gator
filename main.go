package main

import (
  _ "github.com/lib/pq"
	"context"
	"database/sql"
	"fmt"
	"github.com/voylento/gator/internal/config"
	"github.com/voylento/gator/internal/database"
	"github.com/voylento/gator/internal/rss"
	"github.com/google/uuid"
	"html"
	"log"
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

	cmds.Register("addfeed", handleAddFeed)
	cmds.Register("agg", handleAgg)
	cmds.Register("feeds", handleFeeds)
	cmds.Register("login", handleLogin)
	cmds.Register("register", handleRegister)
	cmds.Register("reset", handleReset)
	cmds.Register("users", handleUsers)

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

	_, ok := commands.callbacks[args[1]]
	if !ok {
		fmt.Printf("unknown command: %v\n", args[1])
		return
	}

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

func handleAddFeed(s *State, cmd Command) error {
	if len(cmd.args) != 2 {
		return fmt.Errorf("Usage: addfeed <feedname> <feedurl>")
	}
	
	user, err := s.db.GetUser(context.Background(), s.config.UserName)
	if err != nil {
		log.Fatalf("Failed to read user from db: %v", err)
	}
	
	timeNow := time.Now()
	feedParams := database.CreateFeedParams{
		ID:					uuid.New(),
		CreatedAt:	timeNow,
		UpdatedAt:	timeNow,
		Name:				cmd.args[0],
		Url:				cmd.args[1],
		UserID:			user.ID,
 	}

	feed, err := s.db.CreateFeed(context.Background(), feedParams)
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Create feed %v for %v succeeded\n", cmd.args[0], cmd.args[1])
	fmt.Printf("Feed.ID:\t%v\nFeed.CreatedAt:\t%v\nFeed.UpdatedAt:\t%v\nFeed.Name:\t%v\nFeed.Url:\t%v\nFeed.UserID:\t%v\n", feed.ID, feed.CreatedAt, feed.UpdatedAt, feed.Name, feed.Url, feed.UserID)

	return nil
}

func handleAgg(s *State, cmd Command) error {
	const feedUrl = "https://www.wagslane.dev/index.xml"
	rss, err := rss.FetchFeed(feedUrl)
	if err != nil {
		log.Fatalf("Error calling rss.FetchFeed: %v", err)
	}

	fmt.Printf("Title: %s\n", html.UnescapeString(rss.Channel.Title))
	fmt.Printf("Link: %s\n", html.UnescapeString(rss.Channel.Link))
	fmt.Printf("Description: %s\n", rss.Channel.Description)
	fmt.Println("Items:")
	for _, item := range rss.Channel.Item {
			fmt.Printf("\t- %s (%s)\n", html.UnescapeString(item.Title), item.PubDate)
			fmt.Printf("\t- %s\n", html.UnescapeString(item.Link))
			fmt.Printf("\t- %s\n", html.UnescapeString(item.Description))
	}

	return nil
}

func handleFeeds(s *State, cmd Command) error {
	if len(cmd.args) > 0 {
		return fmt.Errorf("Usage: gator feeds\n")
	}

	feeds, err := s.db.GetAllFeeds(context.Background())
	if err != nil {
		log.Fatalf("Error getting all feeds from db: %v", err)
	}

	for _, feed := range feeds {
		user, err := s.db.GetUserById(context.Background(), feed.UserID)
		if err != nil {
			fmt.Printf("Unable to retrieve user id: %v\n", feed.UserID)
		}
		fmt.Printf("Feed Name: %v\n", feed.Name)
		fmt.Printf("Feed Url: %v\n", feed.Url)
		fmt.Printf("UserName for Feed: %v\n", user.Name)
	}

	return nil
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

	// err = s.db.DeleteAllFeeds(context.Background())
	// if err != nil {
	// 	fmt.Printf("%v\n", err)
	// 	os.Exit(1)
	// }
	//
	// fmt.Println("All feeds deleted from database gator")

	return nil
}

func handleUsers(s *State, cmd Command) error {
	if len(cmd.args) > 0 {
		return fmt.Errorf("Usage: gator users\n")
	}

	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	for _, user := range users {
		if user.Name == s.config.UserName {
			fmt.Printf("* %v (current)\n", user.Name)
		} else {
			fmt.Printf("* %v\n", user.Name)
		}
	}

	return nil
}
