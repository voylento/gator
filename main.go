package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"fmt"
  "github.com/lib/pq"
	"github.com/voylento/gator/internal/config"
	"github.com/voylento/gator/internal/database"
	"github.com/voylento/gator/internal/rss"
	"github.com/google/uuid"
	"html"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"strconv"
	"time"
)

type State struct {
	config 	*config.Config
	db 			*database.Queries
}

type Command struct {
	name		string
	args		[]string
}

// cache for the results of the browse command so that the
// user can open an rss post from the command line
var cachedPosts []database.GetPostsForUserRow

type CommandInfo struct {
	handler func(*State, Command) error
	help string
}

type CommandMap struct {
	commands	map[string]CommandInfo
}

var commandMap *CommandMap

func (c *CommandMap) Run(s *State, cmd Command) error {
	cmdInfo, ok := c.commands[cmd.name]
	if !ok {
		return fmt.Errorf("Unknown command: %s\n", cmd.name)
	}

	err := cmdInfo.handler(s, cmd)
	if err != nil {
		return fmt.Errorf("%w\n", err)
		os.Exit(1)
	}

	return nil
}

func (c *CommandMap) Register(name string, f func(*State, Command) error, helpText string) {
	c.commands[name] = CommandInfo{
		handler: f,
		help: helpText,
	}
}

func (c *CommandMap) GetHelp(commandName string) (string, bool) {
	cmdInfo, ok := c.commands[commandName]
	if !ok {
		return "", false
	}
	return cmdInfo.help, true
}

func (c *CommandMap) GetAllCommands() map[string]string {
	result := make(map[string]string)
	for name, cmdInfo := range c.commands {
		result[name] = cmdInfo.help
	}
	return result
}

func middlewareLoggedIn(handler func(s *State, cmd Command, user database.User) error) func(*State, Command) error{
	return func(s *State, c Command) error {
		user, err := s.db.GetUser(context.Background(), s.config.UserName)
		if err != nil {
			log.Fatalf("Failed to read user from db: %v", err)
		}

		return handler(s, c, user)
	}
}

func InitializeApp() (*State, *CommandMap) {
	cmds := &CommandMap{
		commands: 	make(map[string]CommandInfo),
	}

	commandMap = cmds

	cmds.Register("addfeed", middlewareLoggedIn(handleAddFeed), "addfeed <url> - Add a new RSS feed to follow")
	cmds.Register("agg", handleAgg, "agg <duration) - Aggregate posts from all followed feeds at duration (1s, 1m, 1hr, 5hrs) intervals")
	cmds.Register("allfollows", handleAllFollows, "allfollows - Show all feed follows across all users")
	cmds.Register("browse", middlewareLoggedIn(handleBrowse), "browse [limit] - Browse recent posts (default limit: 2)")
	cmds.Register("feeds", middlewareLoggedIn(handleFeeds), "feeds - List all available feeds")
	cmds.Register("follow", middlewareLoggedIn(handleFollow), "follow <feed_url> - Follow an existing feed")
	cmds.Register("following", middlewareLoggedIn(handleFollowing), "following - List feeds you are following")
	cmds.Register("help", handleHelp, "help [command] - Show help for all commands or a specific command")
	cmds.Register("login", handleLogin, "login <username> - Login as a user")
	cmds.Register("openpost", handleOpenPost, "openpost <post_id> - Open in a post from your last browse command in the browser")
	cmds.Register("register", handleRegister, "register <username> - Create a new user account")
	cmds.Register("reset", handleReset, "reset - reset the database (Warning: Destructive!")
	cmds.Register("unfollow", middlewareLoggedIn(handleUnfollow), "unfollow <feed_url> - Unfollow a feed")
	cmds.Register("users", handleUsers, "users - Show all registered users")

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
	runHandler(state, commands, args[1], commandArgs)
}

func runHandler(s *State, cmdMap *CommandMap, command string, commandArgs []string) error {
	cmd := Command{
		name: command,
		args: commandArgs,
	}

	err := cmdMap.Run(s, cmd)
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	return nil
}

func handleAddFeed(s *State, cmd Command, user database.User) error {
	if len(cmd.args) != 2 {
		if helpText, ok := commandMap.GetHelp("addfeed"); ok {
			fmt.Printf("Usage: %s\n", helpText)
		}
		return nil
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
	fmt.Printf("Feed.ID:\t%v\nFeed.Name:\t%v\nFeed.Url:\t%v\nFeed.UserID:\t%v\n", feed.ID, feed.Name, feed.Url, feed.UserID)

	followFeedArgs := []string{
		cmd.args[1],
	}

	return runHandler(s, commandMap, "follow", followFeedArgs)
}

func handleAgg(s *State, cmd Command) error {
	if len(cmd.args) != 1 {
		if helpText, ok := commandMap.GetHelp("agg"); ok {
			fmt.Printf("Usage: %s\n", helpText)
		}
		return nil
	}

	duration, err := time.ParseDuration(cmd.args[0])
	if err != nil {
		log.Fatalf("Error parsing duration: %v\n", err)
	}

	fmt.Printf("Collecting feeds every %v\n", duration)

	ticker := time.NewTicker(duration)
	for ; ; <-ticker.C {
		scrapeFeeds(s)
	}

	return nil
}

func handleFeeds(s *State, cmd Command, user database.User) error {
	if len(cmd.args) > 0 {
		if helpText, ok := commandMap.GetHelp("feeds"); ok {
			fmt.Printf("Usage: %s\n", helpText)
		}
	}

	feeds, err := s.db.GetAllFeeds(context.Background())
	if err != nil {
		log.Fatalf("Error getting all feeds from db: %v", err)
	}

	for _, feed := range feeds {
		fmt.Printf("Feed Name: %v\n", feed.Name)
		fmt.Printf("Feed Url: %v\n", feed.Url)
		fmt.Printf("UserName for Feed: %v\n", user.Name)
	}

	return nil
}

func handleFollow(s *State, cmd Command, user database.User) error {
	if len(cmd.args) != 1 {
		if helpText, ok := commandMap.GetHelp("follow"); ok {
			fmt.Printf("Usage: %s\n", helpText)
		}
		return nil
	}
	
	feed, err := s.db.GetFeed(context.Background(), cmd.args[0])
	if err != nil {
		log.Printf("feed %v is not in the list of feeds\n", cmd.args[0])
		return nil
	}

	fmt.Printf("handleFollow, user.ID = %v\n", user.ID)
	fmt.Printf("handleFollow, feed.ID = %v\n", feed.ID)
	
	
	timeNow := time.Now()
	feedFollowParams := database.CreateFeedFollowParams{
		ID:					uuid.New(),
		CreatedAt:	timeNow,
		UpdatedAt:	timeNow,
		UserID:			user.ID,
		FeedID:			feed.ID,
 	}

	rows, err := s.db.CreateFeedFollow(context.Background(), feedFollowParams)
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	if len(rows) == 0 {
		log.Fatal("No rows returned after adding a follow for the feed")
	}

	feedFollow := rows[0]

	fmt.Printf("%v is now following %v\n", feedFollow.UserName, feedFollow.FeedName)

	return nil
}

func handleFollowing(s *State, cmd Command, user database.User) error {
	if len(cmd.args) > 0 {
		if helpText, ok := commandMap.GetHelp("following"); ok {
			fmt.Printf("Usage: %s\n", helpText)
		}
		return nil
	}

	fmt.Printf("calling GetFollowsByUser, ID = %v\n", user.ID)
	rows, err := s.db.GetFollowsByUser(context.Background(), user.ID)
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	if len(rows) == 0 {
		fmt.Printf("User %v is not following any rss feeds\n", user.Name)
		return nil
	}

	fmt.Printf("%v is following:\n", user.Name)
	for _, feed := range rows {
		fmt.Printf("%v\n", feed.FeedName)
	}

	return nil
}

func handleAllFollows(s *State, cmd Command) error {
	if len(cmd.args) > 0 {
		if helpText, ok := commandMap.GetHelp("allfollows"); ok {
			fmt.Printf("Usage: %s\n", helpText)
		}
		return nil
	}
	
	rows, err := s.db.GetAllFeedFollows(context.Background())
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	for _, feed := range rows {
		fmt.Println("==========")
		fmt.Printf("ID: %v\n", feed.ID)
		fmt.Printf("UserID: %v\n", feed.UserID)
		fmt.Printf("FeedID: %v\n", feed.FeedID)
	}

	return nil
}

func handleBrowse(s *State, cmd Command, user database.User) error {
	var limit int32
	var err error
	
	if len(cmd.args) > 1 {
		if helpText, ok := commandMap.GetHelp("browse"); ok {
			fmt.Printf("Error: Too many arguments\n")
			fmt.Printf("Usage: %s\n", helpText)
		}
		return nil	
	} else if len(cmd.args) == 1 {
		limit64, err := strconv.ParseInt(cmd.args[0], 10, 32)
		if err != nil{
			fmt.Println("argument to browse must be an integer")
			return nil
		}
		limit = int32(limit64)
	} else {
		limit = 2
	}

	postsForUserParams := database.GetPostsForUserParams {
		UserID: user.ID,
		Limit:	limit,
	}

	rows, err := s.db.GetPostsForUser(context.Background(), postsForUserParams)
	if err != nil {
		fmt.Errorf("Error getting posts for user %v: %v\n", user.ID, err)
	}

	if err := saveCachedPosts(rows); err != nil {
			fmt.Printf("Warning: failed to cache posts: %v\n", err)
			// Continue anyway - not critical
	}
	
	for i, row := range rows {
		fmt.Println("--------------------")
		fmt.Printf("Feed Name: %v\n", row.FeedName)
		fmt.Printf("Title: %v\n", row.Title)
		fmt.Printf("Publish Date: %v\n", row.PublishedAt)
		fmt.Printf("[%d] Url: %v\n", i, row.Url)
		fmt.Printf("Description: %v\n", row.Description)
	}

	return nil
}

func handleOpenPost(s *State, cmd Command) error {
	if len(cmd.args) != 1 {
		if helpText, ok := commandMap.GetHelp("openpost"); ok {
			fmt.Printf("Usage: %s\n", helpText)
		}
		return nil
	} 

	postId64, err := strconv.ParseInt(cmd.args[0], 10, 32)
	if err != nil{
		fmt.Println("argument to openpost must be an integer")
		return nil
	}
	postId := int(postId64)

	// Load cached posts
	cachedPosts, err := loadCachedPosts()
	if err != nil {
			fmt.Println("No cached posts found. Please run 'browse' command first.")
			return nil
	}
	// Check if we have cached posts
	if len(cachedPosts) == 0 {
		fmt.Println("No posts available. Please run 'browse' command first.")
		return nil
	}
	
	// Validate the post ID
	if postId < 0 || postId >= len(cachedPosts) {
		fmt.Printf("Invalid post ID. Please use a number between 0 and %d.\n", len(cachedPosts)-1)
		return nil
	}
	
	// Open the URL in the default browser on macOS
	url := cachedPosts[postId].Url
	fmt.Printf("Opening: %s\n", url)
	
	exec_cmd := exec.Command("open", url)
	err = exec_cmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to open URL: %v", err)
	}

	return nil
}

func handleLogin(s *State, cmd Command) error {
	if len(cmd.args) == 0 {
		if helpText, ok := commandMap.GetHelp("login"); ok {
			fmt.Printf("Usage: %s\n", helpText)
		}
		return nil
	}
	
	user, err := s.db.GetUser(context.Background(), cmd.args[0]) 
	if err != nil {
		log.Fatalf("Failed to read user from db: %v", err)
	}

	s.config.SetUser(user.Name)

	return nil
}

func handleRegister(s *State, cmd Command) error {
	if len(cmd.args) == 0 {
		if helpText, ok := commandMap.GetHelp("register"); ok {
			fmt.Printf("Usage: %s\n", helpText)
		}
		return nil
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
		if helpText, ok := commandMap.GetHelp("reset"); ok {
			fmt.Printf("Usage: %s\n", helpText)
		}
		return nil
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

func scrapeFeeds(s *State) error {
	feed, err := s.db.GetNextFeedToFetch(context.Background())
	if err != nil {
		log.Fatalf("Error getting next feed to fectch: %v\n", err)
	}

	err = s.db.UpdateFeedFetchTime(context.Background(), feed.ID)
	if err != nil {
		log.Fatalf("Error updating feed fetch time: %v\n", err)
	}

	rss, err := rss.FetchFeed(feed.Url)
	if err != nil {
		log.Fatalf("Error fetching rss feed %v\n", err)
	}

	fmt.Println("====================")
	fmt.Printf("%v\n", rss.Channel.Title)
	for _, item := range rss.Channel.Item {
		parsedDate, err := parseRSSDate(item.PubDate)
		if err != nil {
			parsedDate = time.Now()
		}

		escapedTitle := strings.TrimSpace(html.UnescapeString(item.Title))
		escapedUrl := strings.TrimSpace(html.UnescapeString(item.Link))
		escapedDescription := strings.TrimSpace(html.UnescapeString(item.Description))

		if escapedTitle == "" {
			log.Printf("Skipping rss item %v due to blank Title\n", item.Link)
			continue
		}
		if escapedUrl == "" {
			log.Println("Skipping rss item due to blank link")
			continue
		}
		if escapedDescription == "" {
			log.Printf("Skipping rss item %v due to blank Description\n", item.Link)
			continue
		}
		
		postParams := database.CreatePostParams {
			Title: 				escapedTitle,
			Url:					escapedUrl,
			Description: 	escapedDescription,
			PublishedAt:	parsedDate,
			FeedID:      	feed.ID,
		}

		post, err := s.db.CreatePost(context.Background(), postParams)
		fmt.Println("--------------------")
		if err != nil {
			pqErr, ok := err.(*pq.Error)
			if ok && pqErr.Code == "23505" {
				fmt.Printf("Duplicate key, post not saved\n")
				continue
			}
			fmt.Printf("Error creating post: %v\n", err)
		} else {
			fmt.Printf("Post successfully created: %v\n", post.Url)
		}
	}

	return nil
}

func handleUnfollow(s *State, cmd Command, user database.User) error {
	feed, err := s.db.GetFeed(context.Background(), cmd.args[0])
	if err != nil {
		log.Fatalf("feed %v not found\n", cmd.args[0])
	}

	deleteParams := database.DeleteFeedFollowsParams{
		UserID:			user.ID,
		FeedID:			feed.ID,
 	}

	res, err := s.db.DeleteFeedFollows(context.Background(), deleteParams)
	if err != nil {
		log.Fatalf("Error unfollowing feed: %v\n", err)
	}

	fmt.Printf("Unfollow returned %v\n", res)
	return nil
}

func handleHelp(s *State, cmd Command) error {
	if len(cmd.args) == 0 {
			// Show all commands
			fmt.Println("Available commands:")
			fmt.Println("==================")
			
			allCommands := commandMap.GetAllCommands()
			
			// Sort commands alphabetically for consistent output
			var sortedNames []string
			for name := range allCommands {
					sortedNames = append(sortedNames, name)
			}
			sort.Strings(sortedNames)
			
			for _, name := range sortedNames {
					fmt.Printf("  %s\n", allCommands[name])
			}
			
			fmt.Println("\nUse 'help <command>' for detailed help on a specific command.")
			return nil
	}
	
	if len(cmd.args) == 1 {
			// Show help for specific command
			commandName := cmd.args[0]
			helpText, exists := commandMap.GetHelp(commandName)
			if !exists {
					return fmt.Errorf("Unknown command: %s", commandName)
			}
			fmt.Printf("%s\n", helpText)
			return nil
	}
	
	return fmt.Errorf("Usage: help [command]")
}

// Prints all users registered with the application
func handleUsers(s *State, cmd Command) error {
	if len(cmd.args) > 0 {
		if helpText, ok := commandMap.GetHelp("users"); ok {
			fmt.Printf("Usage: %s\n", helpText)
		}
		return nil
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

func parseRSSDate(dateStr string) (time.Time, error) {
	layouts := []string {
		"Mon, 02 Jan 2006 15:04:05 -0700", 	// RFC822 with numeric zone
		"Mon, 02 Jan 2006 15:04:05 MST",  	// RFC822 with named zone
		"2006-01-02T15:04:05Z",							// ISO8601/RFC3339
		"2006-01-02T15:04:05-07:00",				// ISO8601 with offset
		"2006-01-02 15:04:05-0700 MST",		  // Go's time.RFC1123Z format
		"02 Jan 2006 15:04:05 -0700",				// Some other common format
	}

	var parsedTime time.Time
	var err error

	for _, layout := range layouts {
		parsedTime, err = time.Parse(layout, dateStr)
		if err == nil {
			return parsedTime, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date %v\n", dateStr)
}

func getCacheFilePath() string {
    return filepath.Join(os.TempDir(), "gator_posts_cache.json")
}

func saveCachedPosts(posts []database.GetPostsForUserRow) error {
    data, err := json.Marshal(posts)
    if err != nil {
        return err
    }
    return os.WriteFile(getCacheFilePath(), data, 0644)
}

func loadCachedPosts() ([]database.GetPostsForUserRow, error) {
    data, err := os.ReadFile(getCacheFilePath())
    if err != nil {
        return nil, err
    }
    
    var posts []database.GetPostsForUserRow
    err = json.Unmarshal(data, &posts)
    return posts, err
}
