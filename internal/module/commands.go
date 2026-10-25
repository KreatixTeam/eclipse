package module

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// CommandPrefix starts an in-chat proxy command.
const CommandPrefix = "."

// Commands intercepts chat messages beginning with '.' and runs proxy
// commands instead of forwarding them to the server.
//
//	.list            list modules and their state
//	.<module> on|off toggle a module
//	.help            show usage
//
// Other modules can add their own commands via RegisterCommand.
type Commands struct {
	mu      sync.RWMutex
	session *Session
	extra   map[string]extraCommand
}

type extraCommand struct {
	desc string
	fn   func(args []string)
}

func NewCommands() *Commands {
	return &Commands{extra: map[string]extraCommand{}}
}

func (c *Commands) Name() string { return "commands" }

// RegisterCommand adds '.<name>' handled by fn. Call it during OnJoin (or
// earlier); it applies per-session.
func (c *Commands) RegisterCommand(name, desc string, fn func(args []string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.extra[name] = extraCommand{desc: desc, fn: fn}
}

func (c *Commands) OnJoin(s *Session) {
	c.mu.Lock()
	c.session = s
	c.mu.Unlock()
}

func (c *Commands) OnPacket(pk packet.Packet, dir Direction) (packet.Packet, bool) {
	if dir != ServerBound {
		return pk, true
	}
	text, ok := pk.(*packet.Text)
	if !ok || !strings.HasPrefix(text.Message, CommandPrefix) {
		return pk, true
	}
	c.run(text.Message[len(CommandPrefix):])
	return nil, false // swallow the command; never reaches the server
}

func (c *Commands) run(input string) {
	args := strings.Fields(input)
	if len(args) == 0 {
		c.help()
		return
	}
	c.mu.RLock()
	extra, ok := c.extra[args[0]]
	c.mu.RUnlock()
	if ok {
		extra.fn(args[1:])
		return
	}
	switch args[0] {
	case "help":
		c.help()
	case "list":
		c.list()
	default:
		if len(args) != 2 || (args[1] != "on" && args[1] != "off") {
			c.reply("usage: .<module> on|off — try .list or .help")
			return
		}
		if c.session.Registry.Toggle(args[0], args[1] == "on") {
			c.reply(fmt.Sprintf("%s %s", args[0], args[1]))
		} else {
			c.reply("unknown module: " + args[0])
		}
	}
}

func (c *Commands) help() {
	c.mu.RLock()
	extras := make([]string, 0, len(c.extra))
	for n, e := range c.extra {
		extras = append(extras, "."+n+" — "+e.desc)
	}
	c.mu.RUnlock()
	sort.Strings(extras)
	c.reply("eclipse: .list · .<module> on|off" + func() string {
		if len(extras) == 0 {
			return ""
		}
		return " · " + strings.Join(extras, " · ")
	}())
}

func (c *Commands) list() {
	names := c.session.Registry.Names()
	sort.Strings(names)
	parts := make([]string, len(names))
	for i, n := range names {
		state := "off"
		if c.session.Registry.Enabled(n) {
			state = "on"
		}
		parts[i] = fmt.Sprintf("%s (%s)", n, state)
	}
	c.reply(strings.Join(parts, ", "))
}

func (c *Commands) reply(msg string) {
	c.mu.RLock()
	s := c.session
	c.mu.RUnlock()
	if s != nil {
		_ = s.Chat(msg)
	}
}
