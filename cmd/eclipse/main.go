package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"net/http"

	"github.com/KreatixTeam/eclipse/internal/module"
	"github.com/KreatixTeam/eclipse/internal/proxy"
	"github.com/KreatixTeam/eclipse/internal/webui"
)

func main() {
	listen := flag.String("listen", ":19132", "address the proxy listens on (clients join this)")
	server := flag.String("server", "", "upstream Bedrock server address the proxy forwards to (e.g. play.example.com:19132)")
	name := flag.String("name", "Eclipse", "MOTD/name advertised on the LAN ping")
	offline := flag.Bool("offline", false, "disable Xbox Live auth on both ends (LAN/offline servers only)")
	verbose := flag.Bool("v", false, "log every packet flowing through the proxy")
	web := flag.String("web", "", "serve the web dashboard on this address (e.g. :8080)")
	dump := flag.String("dump", "", "log every packet as JSON lines to this file")
	flag.Parse()

	if *server == "" {
		fmt.Fprintln(os.Stderr, "eclipse: -server is required")
		flag.Usage()
		os.Exit(2)
	}

	mods := module.NewRegistry()
	mods.Register(module.NewCommands())
	mods.Register(module.NewPosition())
	mods.Register(module.NewVelocity())
	mods.Register(module.NewFly())
	mods.Toggle("velocity", false)
	mods.Toggle("fly", false)
	if *dump != "" {
		d, err := module.NewDump(*dump)
		if err != nil {
			fmt.Fprintln(os.Stderr, "eclipse: dump:", err)
			os.Exit(1)
		}
		defer d.Close()
		mods.Register(d)
	}
	if *verbose {
		mods.Register(module.NewPacketLogger(os.Stdout))
	}

	p, err := proxy.New(proxy.Config{
		ListenAddr: *listen,
		ServerAddr: *server,
		Name:       *name,
		Offline:    *offline,
		Modules:    mods,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "eclipse:", err)
		os.Exit(1)
	}

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		p.Close()
	}()

	if *web != "" {
		dash := webui.New(mods, p)
		p.SetHooks(dash)
		go func() {
			fmt.Fprintf(os.Stderr, "eclipse: dashboard on http://localhost%s\n", *web)
			if err := http.ListenAndServe(*web, dash.Handler()); err != nil {
				fmt.Fprintln(os.Stderr, "eclipse: webui:", err)
			}
		}()
	}
	if err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "eclipse:", err)
		os.Exit(1)
	}
}
