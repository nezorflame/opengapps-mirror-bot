package config

import (
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	msgEmptyValue = "empty config value '%s'"

	defaultDBPath           = "./bolt.db"
	defaultDBTimeout        = time.Second
	defaultTelegramTimeout  = 60
	defaultGAppsRenewPeriod = time.Minute
)

var mandatoryParams = []string{
	"max_downloads",
	"db.path",
	"db.timeout",
	"gapps.time_format",
	"gapps.prefix",
	"gapps.renew_period",
	"gapps.local_path",
	"gapps.local_url",
	"gapps.local_host",
	"github.repo",
	"github.token",
	"telegram.token",
	"telegram.timeout",
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
		return nil, errors.Wrap(err, "unable to read config")
	}
	cfg.WatchConfig()

	cfg.SetDefault("db.timeout", defaultDBTimeout)
	cfg.SetDefault("gapps.renew_period", defaultGAppsRenewPeriod)
	cfg.SetDefault("telegram.timeout", defaultTelegramTimeout)

	if err := validateConfig(cfg); err != nil {
		return nil, errors.Wrap(err, "unable to validate config")
	}

	return cfg, nil
}

func validateConfig(cfg *viper.Viper) error {
	if cfg == nil {
		return errors.New("config is nil")
	}

	for _, p := range mandatoryParams {
		if cfg.Get(p) == nil {
			return errors.Errorf(msgEmptyValue, p)
		}
	}

	if cfg.GetInt("max_downloads") <= 0 {
		return errors.Errorf("'max_downloads' should be greater than 0")
	}

	if cfg.GetDuration("db.timeout") <= 0 {
		return errors.Errorf("'db.timeout' should be greater than 0")
	}

	if cfg.GetDuration("gapps.renew_period") <= 0 {
		return errors.Errorf("'gapps.renew_period' should be greater than 0")
	}

	if cfg.GetDuration("telegram.timeout") <= 0 {
		return errors.Errorf("'telegram.timeout' should be greater than 0")
	}

	return nil
}
