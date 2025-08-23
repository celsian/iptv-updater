package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gorilla/websocket"
)

type Channel struct {
	Title   string
	ID      string
	Enabled bool
}

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

type ScheduleTask struct {
	ID  string `json:"Id"`
	Key string `json:"Key"`
}

var (
	iptvAPIAddress        string = os.Getenv("IPTV_API_ADDRESS")
	iptvUID               string = os.Getenv("IPTV_UID")
	iptvPass              string = os.Getenv("IPTV_PASS")
	xteveWebSocketAddress string = os.Getenv("XTEVE_WEB_SOCKET_ADDRESS")
	embyAPIAddress        string = os.Getenv("EMBY_API_ADDRESS")
	embyAPIKey            string = os.Getenv("EMBY_API_KEY")
)

func main() {
	fmt.Printf("####### Configuration: #######\n")
	fmt.Printf("iptvAPIAddress: %s\n", iptvAPIAddress)
	printSensitive("iptvUID", iptvUID)
	printSensitive("iptvPass", iptvPass)
	fmt.Printf("xteveWebSocketAddress: %s\n", xteveWebSocketAddress)
	printSensitive("embyAPIKey", embyAPIKey)
	fmt.Printf("embyAPIAddress: %s\n", embyAPIAddress)
	fmt.Printf("##############################\n")

	// 11111111111111111111111111111111111111111111111111111111111111111111
	// Interact with IPTV
	// Get NO_EPG MLB channels
	iptvClient := &http.Client{}
	data := url.Values{}
	data.Set("jxt", "4")
	data.Set("jxw", "sch")
	data.Set("s", "NO_EPG")
	data.Set("c", "MLB")

	req, err := http.NewRequest("POST", iptvAPIAddress, strings.NewReader(data.Encode()))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", fmt.Sprintf("uid=%s; pass=%s", iptvUID, iptvPass))

	resp, err := iptvClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var root map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		panic(err)
	}

	fs := root["Fs"].([]interface{})
	second := fs[1].([]interface{})
	nested := second[1].([]interface{})
	after := nested[1].([]interface{})
	html := after[1].(string)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		panic(err)
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

	for ch, enabled := range channelMap {
		data.Set("jxt", "4")
		data.Set("jxw", "s")
		data.Set("s", "NO_EPG") // Playlist
		data.Set("c", ch.ID)    // Channel ID
		if enabled && !ch.Enabled {
			data.Set("a", "1") // Enable channel with 1
			fmt.Println("Enabling channel: ", ch.Title)
		} else if !enabled {
			data.Set("a", "0") // Disable channel with 0
			fmt.Println("Disabling channel: ", ch.Title)
		}

		req, err := http.NewRequest("POST", iptvAPIAddress, strings.NewReader(data.Encode()))
		if err != nil {
			panic(err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Cookie", fmt.Sprintf("uid=%s; pass=%s", iptvUID, iptvPass))

		resp, err := iptvClient.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
	}

	// 22222222222222222222222222222222222222222222222222222222222222222222
	// Interact with xTeVe
	// 1. Refresh the NO_EPG playlist
	// 2. Get the current list of channels and store it.
	// 3. Update tigers channels: active (true), group (NO_EPG), xmltv file (xteve dummy) and xmltv channel (180 mins)
	xConfigA := getXteveConfig()
	time.Sleep(2 * time.Second) // Wait for xTeVe

	updateM3uFile(xConfigA)
	time.Sleep(2 * time.Second) // Wait for xTeVe

	xConfigB := getXteveConfig()
	time.Sleep(2 * time.Second) // Wait for xTeVe

	updateMapping(xConfigB)
	time.Sleep(2 * time.Second) // Wait for xTeVe

	// 33333333333333333333333333333333333333333333333333333333333333333333
	// Interact with Emby API
	var scheduledTasks []ScheduleTask
	var refreshGuideID string

	scheduledTasksURL := fmt.Sprintf("%s/emby/ScheduledTasks?api_key=%s", embyAPIAddress, embyAPIKey)
	response, err := http.Get(scheduledTasksURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while getting ScheduledTasks: %v\n", err)
		os.Exit(1)
	}
	defer response.Body.Close()

	if err := json.NewDecoder(response.Body).Decode(&scheduledTasks); err != nil {
		panic(err)
	}
	for _, task := range scheduledTasks {
		if task.Key == "RefreshGuide" {
			refreshGuideID = task.ID
			break
		}
	}
	if refreshGuideID == "" {
		fmt.Fprintln(os.Stderr, "Error: could not find task with Key: \"************\"")
		os.Exit(1)
	}
	fmt.Println("Triggering Guide Refresh")
	triggerTaskURL := fmt.Sprintf("%s/emby/ScheduledTasks/Running/%s?api_key=%s", embyAPIAddress, refreshGuideID, embyAPIKey)
	response, err = http.Post(triggerTaskURL, "", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while triggering Refresh Guide: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Channels selected, added and guide refreshed. Ready to watch.")
}

func getXteveConfig() xteveConfig {
	ws, _, err := websocket.DefaultDialer.Dial(xteveWebSocketAddress, nil)
	if err != nil {
		panic(err)
	}
	err = ws.WriteMessage(websocket.TextMessage, []byte(`{"cmd":"getServerConfig"}`))
	if err != nil {
		panic(err)
	}
	_, wsResp, err := ws.ReadMessage()
	if err != nil {
		panic(err)
	}
	ws.Close() // Close first connection

	var xConfig xteveConfig
	if err := json.Unmarshal(wsResp, &xConfig); err != nil {
		panic(err)
	}

	return xConfig
}

func updateM3uFile(xConfig xteveConfig) {
	updateM3u := M3uUpdate{
		Files: Files{
			M3u: make(map[string]M3uFile),
		},
		Command: "updateFileM3U",
	}
	for k, v := range xConfig.Settings.Files.M3u {
		if v.Name == "NO_EPG" {
			updateM3u.Files.M3u[k] = v
			break
		}
	}
	updateM3uPayload, err := json.Marshal(updateM3u)
	if err != nil {
		panic(err)
	}

	ws, _, err := websocket.DefaultDialer.Dial(xteveWebSocketAddress, nil)
	if err != nil {
		panic(err)
	}
	err = ws.WriteMessage(websocket.TextMessage, updateM3uPayload)
	if err != nil {
		panic(err)
	}
	ws.Close()
}

func updateMapping(xConfig xteveConfig) {
	for key, value := range xConfig.Xepg.EpgMapping {
		if !value.XActive && strings.Contains(strings.ToLower(value.Name), "tigers") {
			value.XGroupTitle = "NO_EPG"
			value.XMapping = "180_Minutes"
			value.XXmltvFile = "xTeVe Dummy"
			value.XActive = true

			xConfig.Xepg.EpgMapping[key] = value
			fmt.Printf("Enabling channel: %s\n", value.Name)
		}
	}

	payload := epgMap{
		EpgMapping: xConfig.Xepg.EpgMapping,
		Command:    "saveEpgMapping",
	}

	// &'s get re-encoded when JSON Marshalled, disabling the escape.
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		panic(err)
	}
	saveMappingMessage := buf.Bytes()

	ws, _, err := websocket.DefaultDialer.Dial(xteveWebSocketAddress, nil)
	if err != nil {
		panic(err)
	}
	defer ws.Close()

	err = ws.WriteMessage(websocket.TextMessage, saveMappingMessage)

	if err != nil {
		panic(err)
	}
}

func printSensitive(name, value string) {
	if value != "" {
		fmt.Printf("%s: present\n", name)
	} else {
		fmt.Printf("%s: >>>>>>>>>>>>>>>>>>>> MISSING <<<<<<<<<<<<<<<<<<<<\n", name)
	}
}
