package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"github.com/celsian/iptv-updater/pkg/utils"
	"github.com/go-playground/validator"
	"github.com/joho/godotenv"
)

type Config struct {
	IptvAPIAddress        string `validate:"required"`
	IptvUID               string `validate:"required"`
	IptvPass              string `validate:"required"`
	XteveWebSocketAddress string `validate:"required"`
	EmbyAPIAddress        string `validate:"required"`
	EmbyAPIKey            string `validate:"required"`
}

func Must() *Config {
	logFile := utils.SetupLogging()
	defer logFile.Close()

	// Find where the binary is located
	_, b, _, _ := runtime.Caller(0)
	exeDir := filepath.Dir(filepath.Dir(filepath.Dir(b)))
	fmt.Println("EXE DIR: ", exeDir)

	envPath := filepath.Join(exeDir, ".env")

	// Load environment file
	err := godotenv.Load(envPath)
	if err != nil {
		slog.Error(fmt.Sprintf("Error loading .env file: %v", err))
	}

	cfg := &Config{
		IptvAPIAddress:        os.Getenv("IPTV_API_ADDRESS"),
		IptvUID:               os.Getenv("IPTV_UID"),
		IptvPass:              os.Getenv("IPTV_PASS"),
		XteveWebSocketAddress: os.Getenv("XTEVE_WEB_SOCKET_ADDRESS"),
		EmbyAPIAddress:        os.Getenv("EMBY_API_ADDRESS"),
		EmbyAPIKey:            os.Getenv("EMBY_API_KEY"),
	}

	err = validate(cfg)
	utils.PanicOnErr(err)

	return cfg
}

func validate(cfg *Config) error {
	validate := validator.New()

	if err := validate.Struct(cfg); err != nil {
		return fmt.Errorf("invalid configuration: %v", err)
	}

	slog.Info("Configuration validated, starting task.")
	return nil
}
