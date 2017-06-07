package cmdhandler

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"regexp"

	"github.com/lrstanley/girc"
)

// This may eventually get merged directly into girc in the future.

type Input struct {
	Origin *girc.Event
	Args   []string

	// TODO: utilize Event.Source and PRIVMSG's Params to lookup these to
	// make it easier on the end user.
	User    *girc.User
	Channel *girc.Channel
}

type Command struct {
	Help    string
	MinArgs int
	Fn      func(*girc.Client, *Input)
}

type CmdHandler struct {
	prefix string
	re     *regexp.Regexp

	mu   sync.Mutex
	cmds map[string]*Command
}

var cmdMatch = `^%s([a-z0-9-_]{1,20})(?: (.*))?$`

func New(prefix string) (*CmdHandler, error) {
	re, err := regexp.Compile(fmt.Sprintf(cmdMatch, regexp.QuoteMeta(prefix)))
	if err != nil {
		return nil, err
	}

	return &CmdHandler{prefix: prefix, re: re, cmds: make(map[string]*Command)}, nil
}

var validName = regexp.MustCompile(`^[a-zA-Z0-9-_]{1,20}$`)

func (ch *CmdHandler) Add(name string, cmd *Command) error {
	if cmd == nil {
		return errors.New("nil command provided to CmdHandler")
	}

	name = strings.ToLower(name)

	if !validName.MatchString(name) {
		return fmt.Errorf("invalid command name: %q (req: %q)", name, validName.String())
	}

	if cmd.MinArgs < 0 {
		cmd.MinArgs = 0
	}

	ch.mu.Lock()
	defer ch.mu.Unlock()

	if _, ok := ch.cmds[name]; ok {
		return fmt.Errorf("command already registered: %s", name)
	}

	ch.cmds[name] = cmd

	return nil
}

// Execute satisfies the girc.Handler interface.
func (ch *CmdHandler) Execute(client *girc.Client, event girc.Event) {
	if event.Source == nil || event.Command != girc.PRIVMSG {
		return
	}

	parsed := ch.re.FindStringSubmatch(event.Trailing)
	if len(parsed) != 3 {
		return
	}

	invCmd := strings.ToLower(parsed[1])
	args := strings.Split(parsed[2], " ")
	if len(args) == 1 && args[0] == "" {
		args = []string{}
	}

	ch.mu.Lock()
	defer ch.mu.Unlock()

	if invCmd == "help" {
		if len(args) == 0 {
			client.Commands.ReplyTo(event, girc.Fmt("type '{b}!help {blue}<command>{c}{b}' to optionally get more info about a specific command."))
			return
		}

		args[0] = strings.ToLower(args[0])

		if _, ok := ch.cmds[args[0]]; !ok {
			client.Commands.ReplyTof(event, girc.Fmt("unknown command {b}%q{b}."), args[0])
			return
		}

		if ch.cmds[args[0]].Help == "" {
			client.Commands.ReplyTof(event, girc.Fmt("there is no help documentaiton for {b}%q{b}"), args[0])
			return
		}

		client.Commands.ReplyTof(event, girc.Fmt("{b}%s%s{b} :: "+ch.cmds[args[0]].Help), ch.prefix, args[0])
		return
	}

	cmd, ok := ch.cmds[invCmd]
	if !ok {
		// TODO: return "no command found" if config option set to do so?
		return
	}

	if len(args) < cmd.MinArgs {
		client.Commands.ReplyTof(event, girc.Fmt("not enough arguments supplied for {b}%q{b}. try '{b}%shelp %s{b}'?"), invCmd, ch.prefix, invCmd)
		return
	}

	in := &Input{
		Origin: &event,
		Args:   args,
	}

	go cmd.Fn(client, in)
}
