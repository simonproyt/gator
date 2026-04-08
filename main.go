package main

import (
	"fmt"
	"log"
	"os"

	"gator/internal/config"
)

type state struct {
	cfg *config.Config
}

type command struct {
	name string
	args []string
}

type commands struct {
	handlers map[string]func(*state, command) error
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
	if err := s.cfg.SetUser(username); err != nil {
		return err
	}
	fmt.Printf("current user set to %s\n", username)
	return nil
}

func main() {
	cfg, err := config.Read()
	if err != nil {
		log.Fatalf("read config: %v", err)
	}

	s := &state{cfg: &cfg}

	cmds := &commands{handlers: make(map[string]func(*state, command) error)}
	cmds.register("login", handlerLogin)

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
