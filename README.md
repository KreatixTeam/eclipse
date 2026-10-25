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
- `commands` module — in-chat control: `.list`, `.<module> on|off`, `.help`,
  extensible by other modules (`.pos`)
- `position` module — tracks your coordinates, `.pos` prints them
- `velocity` module — anti-knockback: zeroes SetActorMotion aimed at you
- `fly` module — forces flight abilities client-side (`.fly on|off`)
- `dump` module — `-dump packets.jsonl` records every packet as JSON lines
- Embedded web dashboard (`-web :8080`): Vue 3 + Vuetify OLED-dark UI with
  live module toggles, players and a streaming packet feed (SSE)
- Xbox Live auth toward upstream servers via device-code sign-in
  (`-offline` for authless/LAN servers)

## Build & run

```sh
make          # builds web/dist (npm) then the go binary
# or just:    go build -o eclipse ./cmd/eclipse   (dashboard needs `make web` first)

# LAN / offline server, with the web dashboard
./eclipse -listen :19132 -server 127.0.0.1:19133 -offline -v -web :8080

# Real server (one-time Microsoft device-code sign-in printed to stderr)
./eclipse -listen :19132 -server play.geysermc.org:19132
```

Then join `:19132` (or pick Eclipse from the LAN list) in Minecraft Bedrock.

## Hosted hub (eclipsed)

`eclipsed` runs Eclipse as a multi-tenant service: each user signs in with
their own Microsoft/Xbox account via device code, adds servers, and gets a
dedicated proxy port to join from Minecraft Bedrock.

```bash
make hosted
./eclipsed -http 127.0.0.1:8920 -data data -port-from 25000 -port-to 25099
```

Serve the HTTP endpoint behind TLS and keep the port range open on UDP.
Live at https://eclipse.kreatix.dev.

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
