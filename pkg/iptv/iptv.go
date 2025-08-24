package iptv

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/celsian/iptv-updater/pkg/config"
	"github.com/celsian/iptv-updater/pkg/utils"
)

type Channel struct {
	Title   string
	ID      string
	Enabled bool
}

func Update(cfg *config.Config) {
	// 11111111111111111111111111111111111111111111111111111111111111111111
	// Interact with IPTV
	// Get NO_EPG MLB channels
	slog.Info("Querying IPTV for channels...")

	iptvClient := &http.Client{}
	data := url.Values{}
	data.Set("jxt", "4")
	data.Set("jxw", "sch")
	data.Set("s", "NO_EPG")
	data.Set("c", "MLB")

	req, err := http.NewRequest("POST", cfg.IptvAPIAddress, strings.NewReader(data.Encode()))
	if err != nil {
		utils.PanicOnErr(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", fmt.Sprintf("uid=%s; pass=%s", cfg.IptvUID, cfg.IptvPass))

	resp, err := iptvClient.Do(req)
	if err != nil {
		utils.PanicOnErr(err)
	}
	defer resp.Body.Close()

	var root map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		utils.PanicOnErr(err)
	}

	fs := root["Fs"].([]interface{})
	second := fs[1].([]interface{})
	nested := second[1].([]interface{})
	after := nested[1].([]interface{})
	html := after[1].(string)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		utils.PanicOnErr(err)
	}

	var channels []Channel

	// Each <li> is a channel entry
	doc.Find("li").Each(func(i int, s *goquery.Selection) {
		input := s.Find("input[type=checkbox]")
		title := strings.TrimSpace(s.Find("span").First().Text())

		if input.Length() > 0 {
			id, _ := input.Attr("id")
			_, checked := input.Attr("checked")

			channels = append(channels, Channel{
				Title:   title,
				ID:      id,
				Enabled: checked,
			})
		}
	})

	var channelMap = map[Channel]bool{}

	for _, ch := range channels {
		if ch.Enabled {
			if !(strings.Contains(ch.Title, "US MLB San Diego Padres") || strings.Contains(ch.Title, "US MLB Network")) {
				channelMap[ch] = false
			}
		}
		if strings.Contains(strings.ToLower(ch.Title), "tigers") {
			channelMap[ch] = true
		}
	}

	counter := 0
	for ch, enabled := range channelMap {
		data.Set("jxt", "4")
		data.Set("jxw", "s")
		data.Set("s", "NO_EPG") // Playlist
		data.Set("c", ch.ID)    // Channel ID
		if enabled && !ch.Enabled {
			data.Set("a", "1") // Enable channel with 1
			slog.Info(fmt.Sprintf("IPTV: Enabling channel: %s", ch.Title))
			counter++
		} else if !enabled {
			data.Set("a", "0") // Disable channel with 0
			slog.Info(fmt.Sprintf("IPTV: Disabling channel: %s", ch.Title))
			counter++
		}

		req, err := http.NewRequest("POST", cfg.IptvAPIAddress, strings.NewReader(data.Encode()))
		if err != nil {
			utils.PanicOnErr(err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Cookie", fmt.Sprintf("uid=%s; pass=%s", cfg.IptvUID, cfg.IptvPass))

		resp, err := iptvClient.Do(req)
		if err != nil {
			utils.PanicOnErr(err)
		}
		defer resp.Body.Close()
	}

	if counter == 0 {
		slog.Info("Exiting: IPTV: No channels to change.")
		os.Exit(0)
	}
}
