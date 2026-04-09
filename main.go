package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gator/internal/config"
	"gator/internal/database"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type state struct {
	db  *database.Queries
	cfg *config.Config
}

type command struct {
	name string
	args []string
}

type commands struct {
	handlers map[string]func(*state, command) error
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

func (c *commands) register(name string, f func(*state, command) error) {
	c.handlers[name] = f
}

func (c *commands) run(s *state, cmd command) error {
	if f, ok := c.handlers[cmd.name]; ok {
		return f(s, cmd)
	}
	return fmt.Errorf("unknown command: %s", cmd.name)
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("username required")
	}
	username := cmd.args[0]
	ctx := context.Background()
	if _, err := s.db.GetUser(ctx, username); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("user does not exist")
		}
		return err
	}
	if err := s.cfg.SetUser(username); err != nil {
		return err
	}
	fmt.Printf("current user set to %s\n", username)
	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("username required")
	}
	name := cmd.args[0]
	ctx := context.Background()
	// ensure user doesn't already exist
	if _, err := s.db.GetUser(ctx, name); err == nil {
		return fmt.Errorf("user already exists")
	} else if err != sql.ErrNoRows {
		return err
	}

	now := time.Now().UTC()
	user, err := s.db.CreateUser(ctx, database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		Name:      name,
	})
	if err != nil {
		return err
	}
	if err := s.cfg.SetUser(name); err != nil {
		return err
	}
	fmt.Printf("user created: %s\n", name)
	log.Printf("created user: %+v\n", user)
	return nil
}

func handlerReset(s *state, cmd command) error {
	ctx := context.Background()
	if err := s.db.DeleteUsers(ctx); err != nil {
		return err
	}
	fmt.Println("database reset")
	return nil
}

func handlerUsers(s *state, cmd command) error {
	ctx := context.Background()
	users, err := s.db.GetUsers(ctx)
	if err != nil {
		return err
	}
	for _, user := range users {
		if user.Name == s.cfg.CurrentUserName {
			fmt.Printf("* %s (current)\n", user.Name)
			continue
		}
		fmt.Printf("* %s\n", user.Name)
	}
	return nil
}

func handlerAddFeed(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 2 {
		return fmt.Errorf("name and url required")
	}
	name := cmd.args[0]
	url := cmd.args[1]
	ctx := context.Background()
	now := time.Now().UTC()
	feed, err := s.db.CreateFeed(ctx, database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		Name:      name,
		Url:       url,
		UserID:    uuid.NullUUID{UUID: user.ID, Valid: true},
	})
	if err != nil {
		return err
	}
	// automatically follow the feed for the creator
	followRow, err := s.db.CreateFeedFollow(ctx, database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		UserID:    user.ID,
		FeedID:    feed.ID,
	})
	if err != nil {
		return err
	}
	fmt.Printf("%s %s\n", followRow.FeedName, followRow.UserName)
	return nil
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		if s.cfg.CurrentUserName == "" {
			return fmt.Errorf("no current user set")
		}
		ctx := context.Background()
		user, err := s.db.GetUser(ctx, s.cfg.CurrentUserName)
		if err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("current user does not exist")
			}
			return err
		}
		return handler(s, cmd, user)
	}
}

func handlerFeedsList(s *state, cmd command) error {
	ctx := context.Background()
	rows, err := s.db.GetFeeds(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		owner := ""
		if r.OwnerName.Valid {
			owner = r.OwnerName.String
		}
		fmt.Printf("* %s (%s) - %s\n", r.Name, r.Url, owner)
	}
	return nil
}

func handlerFollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("url required")
	}
	url := cmd.args[0]
	ctx := context.Background()
	feed, err := s.db.GetFeedByURLForFollow(ctx, url)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	row, err := s.db.CreateFeedFollow(ctx, database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		UserID:    user.ID,
		FeedID:    feed.ID,
	})
	if err != nil {
		return err
	}
	fmt.Printf("%s %s\n", row.FeedName, row.UserName)
	return nil
}

func handlerUnfollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("url required")
	}
	url := cmd.args[0]
	ctx := context.Background()
	feed, err := s.db.GetFeedByURLForFollow(ctx, url)
	if err != nil {
		return err
	}
	if err := s.db.DeleteFeedFollow(ctx, database.DeleteFeedFollowParams{UserID: user.ID, FeedID: feed.ID}); err != nil {
		return err
	}
	fmt.Printf("unfollowed %s %s\n", feed.Name, user.Name)
	return nil
}

func handlerFollowing(s *state, cmd command, user database.User) error {
	ctx := context.Background()
	rows, err := s.db.GetFeedFollowsForUser(ctx, user.ID)
	if err != nil {
		return err
	}
	for _, r := range rows {
		fmt.Printf("* %s\n", r.FeedName)
	}
	return nil
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "gator")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var feed RSSFeed
	if err := xml.Unmarshal(b, &feed); err != nil {
		return nil, err
	}
	// Unescape HTML entities for titles and descriptions
	feed.Channel.Title = html.UnescapeString(feed.Channel.Title)
	feed.Channel.Description = html.UnescapeString(feed.Channel.Description)
	for i := range feed.Channel.Item {
		feed.Channel.Item[i].Title = html.UnescapeString(feed.Channel.Item[i].Title)
		feed.Channel.Item[i].Description = html.UnescapeString(feed.Channel.Item[i].Description)
	}
	return &feed, nil
}

func handlerAgg(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("time_between_reqs required")
	}
	d, err := time.ParseDuration(cmd.args[0])
	if err != nil {
		return err
	}
	fmt.Printf("Collecting feeds every %s\n", d)

	scrapeOnce := func() {
		if err := scrapeFeeds(s); err != nil {
			log.Printf("scrape error: %v", err)
		}
	}

	// run immediately
	scrapeOnce()

	ticker := time.NewTicker(d)
	defer ticker.Stop()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-sig:
			fmt.Println("stopping aggregator")
			return nil
		case <-ticker.C:
			scrapeOnce()
		}
	}
}

func scrapeFeeds(s *state) error {
	ctx := context.Background()
	feed, err := s.db.GetNextFeedToFetch(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	if err := s.db.MarkFeedFetched(ctx, feed.ID); err != nil {
		return err
	}
	rss, err := fetchFeed(ctx, feed.Url)
	if err != nil {
		log.Printf("fetch error for %s: %v", feed.Url, err)
		return nil
	}
	fmt.Printf("Feed: %s (%s)\n", feed.Name, feed.Url)
	for _, item := range rss.Channel.Item {
		fmt.Printf("- %s\n", item.Title)
	}
	return nil
}

func main() {
	cfg, err := config.Read()
	if err != nil {
		log.Fatalf("read config: %v", err)
	}

	db, err := sql.Open("postgres", cfg.DbURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	dbQueries := database.New(db)

	s := &state{cfg: &cfg, db: dbQueries}

	cmds := &commands{handlers: make(map[string]func(*state, command) error)}
	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegister)
	cmds.register("reset", handlerReset)
	cmds.register("users", handlerUsers)
	cmds.register("agg", handlerAgg)
	cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	cmds.register("feeds", handlerFeedsList)
	cmds.register("follow", middlewareLoggedIn(handlerFollow))
	cmds.register("following", middlewareLoggedIn(handlerFollowing))
	cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "not enough arguments were provided")
		os.Exit(1)
	}

	name := os.Args[1]
	args := []string{}
	if len(os.Args) > 2 {
		args = os.Args[2:]
	}

	cmd := command{name: name, args: args}
	if err := cmds.run(s, cmd); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
