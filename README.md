# Eclipse

A free, open-source Minecraft: Bedrock Edition **man-in-the-middle proxy** and
module client — the foss answer to paid, closed-source tools like Lunar Proxy.

Eclipse sits between your game and a server: clients (PC, mobile, console on
the same network) join it like any other server over LAN, and every packet is
forwarded through a module pipeline that can inspect, modify or inject traffic.

## Status

Early scaffold. Working so far:

- MITM bridge (`-listen` accepts clients, `-server` forwards upstream)
- LAN advertisement so consoles/phones see it in the Friends/LAN tab
- Module registry with per-packet hooks in both directions
- `packet-logger` module (`-v`)
- Xbox Live auth toward upstream servers via device-code sign-in
  (`-offline` for authless/LAN servers)

## Build & run

```sh
go build ./cmd/eclipse

# LAN / offline server
./eclipse -listen :19132 -server 127.0.0.1:19133 -offline -v

# Real server (one-time Microsoft device-code sign-in printed to stderr)
./eclipse -listen :19132 -server play.geysermc.org:19132
```

Then join `:19132` (or pick Eclipse from the LAN list) in Minecraft Bedrock.

## Writing a module

```go
type Module interface {
    Name() string
    OnPacket(pk packet.Packet, dir Direction) (packet.Packet, bool)
}
```

Return the (possibly modified) packet and `true` to forward it, or `nil, false`
to drop it. Register it in `cmd/eclipse/main.go` with `mods.Register(...)`.

## License

MIT — see [LICENSE](LICENSE).
