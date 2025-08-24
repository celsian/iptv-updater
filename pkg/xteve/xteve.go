package xteve

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/celsian/iptv-updater/pkg/config"
	"github.com/celsian/iptv-updater/pkg/utils"
	"github.com/gorilla/websocket"
)

type EPGEntry struct {
	FileM3uID          string `json:"_file.m3u.id"`
	FileM3uName        string `json:"_file.m3u.name"`
	FileM3uPath        string `json:"_file.m3u.path"`
	GroupTitle         string `json:"group-title"`
	Name               string `json:"name"`
	TvgId              string `json:"tvg-id"`
	TvgLogo            string `json:"tvg-logo"`
	TvgName            string `json:"tvg-name"`
	Url                string `json:"url"`
	UuidKey            string `json:"_uuid.key"`
	Values             string `json:"_values"`
	XActive            bool   `json:"x-active"`
	XCategory          string `json:"x-category"`
	XChannelID         string `json:"x-channelID"`
	XEpg               string `json:"x-epg"`
	XGroupTitle        string `json:"x-group-title"`
	XMapping           string `json:"x-mapping"`
	XXmltvFile         string `json:"x-xmltv-file"`
	XName              string `json:"x-name"`
	XUpdateChannelIcon bool   `json:"x-update-channel-icon"`
	XUpdateChannelName bool   `json:"x-update-channel-name"`
	XDescription       string `json:"x-description"`
}

type epgMap struct {
	EpgMapping map[string]EPGEntry `json:"epgMapping"`
	Command    string              `json:"cmd"`
}

type M3uFile struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	FileSource  string `json:"file.source"`
	Tuner       int    `json:"tuner"`
}

type Files struct {
	M3u map[string]M3uFile `json:"m3u"`
}

type M3uUpdate struct {
	Files   Files  `json:"files"`
	Command string `json:"cmd"`
}

type settings struct {
	Files struct {
		M3u map[string]M3uFile `json:"m3u"`
	} `json:"files"`
}

type xteveConfig struct {
	Xepg     epgMap   `json:"xepg"`
	Settings settings `json:"settings"`
}

type xteve struct {
	cfg  *config.Config
	xCfg *xteveConfig
}

func Update(cfg *config.Config) {
	x := xteve{cfg: cfg}

	slog.Info("xTeVe: Get initial config...")
	x.getXteveConfig()
	time.Sleep(2 * time.Second) // Wait for xTeVe

	slog.Info("xTeVe: Update M3U playlist...")
	x.updateM3uFile()
	time.Sleep(2 * time.Second) // Wait for xTeVe

	slog.Info("xTeVe: Get updated config...")
	x.getXteveConfig()
	time.Sleep(2 * time.Second) // Wait for xTeVe

	slog.Info("xTeVe: Enable channel mapping...")
	x.updateMapping()
	time.Sleep(2 * time.Second) // Wait for xTeVe
}

func (x *xteve) getXteveConfig() {
	ws, _, err := websocket.DefaultDialer.Dial(x.cfg.XteveWebSocketAddress, nil)
	if err != nil {
		utils.PanicOnErr(err)
	}
	err = ws.WriteMessage(websocket.TextMessage, []byte(`{"cmd":"getServerConfig"}`))
	if err != nil {
		utils.PanicOnErr(err)
	}
	_, wsResp, err := ws.ReadMessage()
	if err != nil {
		utils.PanicOnErr(err)
	}
	ws.Close() // Close first connection

	var xConfig xteveConfig
	if err := json.Unmarshal(wsResp, &xConfig); err != nil {
		utils.PanicOnErr(err)
	}

	x.xCfg = &xConfig
}

func (x *xteve) updateM3uFile() {
	updateM3u := M3uUpdate{
		Files: Files{
			M3u: make(map[string]M3uFile),
		},
		Command: "updateFileM3U",
	}
	for k, v := range x.xCfg.Settings.Files.M3u {
		if v.Name == "NO_EPG" {
			updateM3u.Files.M3u[k] = v
			break
		}
	}
	updateM3uPayload, err := json.Marshal(updateM3u)
	if err != nil {
		utils.PanicOnErr(err)
	}

	ws, _, err := websocket.DefaultDialer.Dial(x.cfg.XteveWebSocketAddress, nil)
	if err != nil {
		utils.PanicOnErr(err)
	}
	err = ws.WriteMessage(websocket.TextMessage, updateM3uPayload)
	if err != nil {
		utils.PanicOnErr(err)
	}
	ws.Close()
}

func (x *xteve) updateMapping() {
	for key, value := range x.xCfg.Xepg.EpgMapping {
		if !value.XActive && strings.Contains(strings.ToLower(value.Name), "tigers") {
			value.XGroupTitle = "NO_EPG"
			value.XMapping = "180_Minutes"
			value.XXmltvFile = "xTeVe Dummy"
			value.XActive = true

			x.xCfg.Xepg.EpgMapping[key] = value
			slog.Info(fmt.Sprintf("xTeVe: Enabling channel: %s", value.Name))
		}
	}

	payload := epgMap{
		EpgMapping: x.xCfg.Xepg.EpgMapping,
		Command:    "saveEpgMapping",
	}

	// &'s get re-encoded when JSON Marshalled, disabling the escape.
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		utils.PanicOnErr(err)
	}
	saveMappingMessage := buf.Bytes()

	ws, _, err := websocket.DefaultDialer.Dial(x.cfg.XteveWebSocketAddress, nil)
	if err != nil {
		utils.PanicOnErr(err)
	}
	defer ws.Close()

	err = ws.WriteMessage(websocket.TextMessage, saveMappingMessage)

	if err != nil {
		utils.PanicOnErr(err)
	}
}
