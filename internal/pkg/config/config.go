package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/viper"
)

const (
	msgEmptyValue = "empty config value '%s'"

	defaultDBPath           = "./bolt.db"
	defaultDBTimeout        = time.Second
	defaultTelegramTimeout  = 60
	defaultTelegramDebug    = false
	defaultGAppsRenewPeriod = time.Minute
)

var mandatoryParams = []string{
	"max_downloads",
	"gapps.time_format",
	"gapps.prefix",
	"gapps.local_path",
	"gapps.local_url",
	"gapps.local_host",
	"github.repo",
	"github.token",
	"telegram.token",
	"commands.start",
	"commands.help",
	"commands.mirror",
	"messages.hello",
	"messages.help",
	"messages.mirror.in_progress",
	"messages.mirror.found",
	"messages.mirror.not_found",
	"messages.mirror.missing",
	"messages.mirror.ok",
	"messages.mirror.fail",
	"messages.errors.platform",
	"messages.errors.android",
	"messages.errors.variant",
	"messages.errors.date",
	"messages.errors.mirror",
	"messages.errors.unknown",
}

// New creates new viper config instance
func New(name string) (*viper.Viper, error) {
	if name == "" {
		return nil, errors.New("empty config name")
	}

	cfg := viper.New()

	cfg.SetConfigName(name)
	cfg.SetConfigType("toml")
	cfg.AddConfigPath("$HOME/.config")
	cfg.AddConfigPath("/etc")
	cfg.AddConfigPath(".")

	if err := cfg.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("unable to read config: %w", err)
	}
	cfg.WatchConfig()

	cfg.SetDefault("db.path", defaultDBPath)
	cfg.SetDefault("db.timeout", defaultDBTimeout)
	cfg.SetDefault("gapps.renew_period", defaultGAppsRenewPeriod)
	cfg.SetDefault("telegram.timeout", defaultTelegramTimeout)
	cfg.SetDefault("telegram.debug", defaultTelegramDebug)

	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("unable to validate config: %w", err)
	}

	return cfg, nil
}

func validateConfig(cfg *viper.Viper) error {
	if cfg == nil {
		return errors.New("config is nil")
	}

	for _, p := range mandatoryParams {
		if cfg.Get(p) == nil {
			return fmt.Errorf(msgEmptyValue, p)
		}
	}

	if cfg.GetInt("max_downloads") <= 0 {
		return errors.New("'max_downloads' should be greater than 0")
	}

	if cfg.GetDuration("db.timeout") <= 0 {
		return errors.New("'db.timeout' should be greater than 0")
	}

	if cfg.GetDuration("gapps.renew_period") <= 0 {
		return errors.New("'gapps.renew_period' should be greater than 0")
	}

	if cfg.GetDuration("telegram.timeout") <= 0 {
		return errors.New("'telegram.timeout' should be greater than 0")
	}

	return nil
}
