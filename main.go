package main

import (
	"golang.org/x/exp/slog"

	"github.com/celsian/iptv-updater/pkg/config"
	"github.com/celsian/iptv-updater/pkg/emby"
	"github.com/celsian/iptv-updater/pkg/iptv"
	"github.com/celsian/iptv-updater/pkg/xteve"
)

func main() {
	// Build config from environment
	cfg, logFileCloser := config.Must()
	defer logFileCloser()

	// Interact with IPTV
	iptv.Update(cfg)

	// Interact with xTeVe
	xteve.Update(cfg)

	// Interact with Emby API
	emby.RefreshGuide(cfg)

	slog.Info("Exiting: Task Complete. Channels selected and added, guide refreshed. Ready to watch.")
}
