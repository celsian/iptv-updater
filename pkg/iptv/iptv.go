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

type Iptv struct {
	cfg        *config.Config
	httpClient *http.Client
}

func New(cfg *config.Config) *Iptv {
	return &Iptv{
		cfg:        cfg,
		httpClient: &http.Client{},
	}
}

func (i *Iptv) Update() {
	slog.Info("Querying IPTV for channels...")

	// Get NO_EPG MLB channels
	data := url.Values{
		"jxt": {"4"},
		"jxw": {"sch"},
		"s":   {"NO_EPG"},
		"c":   {"MLB"},
	}

	jsonBody := i.requestJson(data)

	channels := i.parseChannels(jsonBody)

	i.updateChannels(channels)
}

func (i *Iptv) requestJson(data url.Values) (jsonBody map[string]interface{}) {
	req, err := http.NewRequest("POST", i.cfg.IptvAPIAddress, strings.NewReader(data.Encode()))
	if err != nil {
		utils.PanicOnErr(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", fmt.Sprintf("uid=%s; pass=%s", i.cfg.IptvUID, i.cfg.IptvPass))

	resp, err := i.httpClient.Do(req)
	if err != nil {
		utils.PanicOnErr(err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		utils.PanicOnErr(err)
	}

	return
}

func (i *Iptv) parseChannels(jsonBody map[string]interface{}) (channels []Channel) {
	fs := jsonBody["Fs"].([]interface{})
	second := fs[1].([]interface{})
	nested := second[1].([]interface{})
	after := nested[1].([]interface{})
	html := after[1].(string)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		utils.PanicOnErr(err)
	}

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

	return
}

func (i *Iptv) updateChannels(channels []Channel) {
	var channelMap = map[Channel]bool{}

	for _, ch := range channels {
		if ch.Enabled && !utils.ContainsSlice(ch.Title, []string{"US MLB Network", "US MLB San Diego Padres", "tigers"}) {
			channelMap[ch] = false
		} else if !ch.Enabled && utils.ContainsSlice(ch.Title, []string{"tigers"}) {
			channelMap[ch] = true
		}
	}

	if len(channelMap) == 0 {
		slog.Info("Exiting: IPTV: No channels to change.")
		os.Exit(0)
	}

	for ch, enabled := range channelMap {
		data := url.Values{
			"jxt": {"4"},
			"jxw": {"s"},
			"s":   {"NO_EPG"}, // Playlist
			"c":   {ch.ID},    // Channel ID
		}
		if enabled {
			data.Set("a", "1") // Enable channel with 1
			slog.Info(fmt.Sprintf("IPTV: Enabling channel: %s", ch.Title))
		} else {
			data.Set("a", "0") // Disable channel with 0
			slog.Info(fmt.Sprintf("IPTV: Disabling channel: %s", ch.Title))
		}

		_ = i.requestJson(data)
	}
}
