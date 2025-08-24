package emby

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/celsian/iptv-updater/pkg/config"
	"github.com/celsian/iptv-updater/pkg/utils"
)

type ScheduleTask struct {
	ID  string `json:"Id"`
	Key string `json:"Key"`
}

func RefreshGuide(cfg *config.Config) {
	var scheduledTasks []ScheduleTask
	var refreshGuideID string

	scheduledTasksURL := fmt.Sprintf("%s/emby/ScheduledTasks?api_key=%s", cfg.EmbyAPIAddress, cfg.EmbyAPIKey)
	response, err := http.Get(scheduledTasksURL)
	if err != nil {
		slog.Error(fmt.Sprintf("Emby: Error while getting ScheduledTasks: %v", err))
		os.Exit(1)
	}
	defer response.Body.Close()

	if err := json.NewDecoder(response.Body).Decode(&scheduledTasks); err != nil {
		utils.PanicOnErr(err)
	}
	for _, task := range scheduledTasks {
		if task.Key == "RefreshGuide" {
			refreshGuideID = task.ID
			break
		}
	}
	if refreshGuideID == "" {
		slog.Error("Emby: Error: could not find task with Key: \"************\"")
		os.Exit(1)
	}
	slog.Info("Emby: Triggering Guide Refresh")
	triggerTaskURL := fmt.Sprintf("%s/emby/ScheduledTasks/Running/%s?api_key=%s", cfg.EmbyAPIAddress, refreshGuideID, cfg.EmbyAPIKey)
	response, err = http.Post(triggerTaskURL, "", nil)
	if err != nil {
		slog.Error(fmt.Sprintf("Emby: Error while triggering Refresh Guide: %v", err))
		os.Exit(1)
	}
}
