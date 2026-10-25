package module

import (
	"fmt"
	"sort"
	"strings"

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
type Commands struct {
	session *Session
}

func NewCommands() *Commands { return &Commands{} }

func (c *Commands) Name() string { return "commands" }

func (c *Commands) OnJoin(s *Session) { c.session = s }

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
	switch args[0] {
	case "help":
		c.help()
	case "list":
		c.list()
	default:
		if len(args) != 2 || (args[1] != "on" && args[1] != "off") {
			c.reply("usage: .<module> on|off — try .list")
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
	c.reply("eclipse: .list · .<module> on|off · .help")
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
	if c.session != nil {
		_ = c.session.Chat(msg)
	}
}
