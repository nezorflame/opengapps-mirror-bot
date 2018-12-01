package config

import (
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// Config is used for app configuration
type Config struct {
	EnableTracing bool
	EnableDebug   bool
	MaxDownloads  uint

	GAppsTimeFormat     string
	GAppsPrefix         string
	GAppsLocalPath      string
	GAppsLocalURL       string
	GAppsLocalHostname  string
	GAppsRemoteURL      string
	GAppsRemoteHostname string

	GithubRepo  string
	GithubToken string

	TelegramToken   string
	TelegramTimeout int

	MsgHello            string
	MsgHelp             string
	MsgMirrorInProgress string
	MsgMirrorFound      string
	MsgMirrorNotFound   string
	MsgMirrorMissing    string
	MsgMirrorOK         string
	MsgMirrorFail       string

	MsgErrPlatform string
	MsgErrAndroid  string
	MsgErrVariant  string
	MsgErrDate     string
	MsgErrMirror   string
	MsgErrUnknown  string
}

const (
	emptyErr = "'%s' parameter is empty"
	parseErr = "'%s' parameter is invalid"
)

// Init creates new instance of Config
func Init(name string) (*Config, error) {
	viper.SetConfigName(name)
	viper.AddConfigPath("/etc/")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		return nil, errors.Wrap(err, "unable to read config")
	}

	c := &Config{}

	c.EnableTracing = viper.GetBool("tracing")
	c.EnableDebug = viper.GetBool("debug")
	maxDownloads := viper.GetInt("max_downloads")
	if maxDownloads <= 0 {
		return nil, errors.Errorf(emptyErr, "max_downloads")
	}
	c.MaxDownloads = uint(maxDownloads)

	gappsSection := viper.Sub("gapps")
	if c.GAppsTimeFormat = gappsSection.GetString("time_format"); c.GAppsTimeFormat == "" {
		return nil, errors.Errorf(emptyErr, "gapps.time_format")
	}
	if c.GAppsPrefix = gappsSection.GetString("prefix"); c.GAppsPrefix == "" {
		return nil, errors.Errorf(emptyErr, "gapps.prefix")
	}

	if c.GAppsLocalPath = gappsSection.GetString("local_path"); c.GAppsLocalPath != "" {
		if _, err := os.Stat(c.GAppsLocalPath); os.IsNotExist(err) {
			return nil, errors.New("folder on path 'gapps.local_path' does not exist")
		}
	}

	if c.GAppsLocalURL = gappsSection.GetString("local_url"); c.GAppsLocalURL != "" {
		if c.GAppsLocalPath == "" {
			return nil, errors.New("you must provide 'gapps.local_path' along with 'gapps.local_url'")
		}
		if c.GAppsLocalHostname = gappsSection.GetString("local_host"); c.GAppsLocalHostname == "" {
			return nil, errors.New("you must provide 'gapps.local_host' along with 'gapps.local_url'")
		}
	}

	if c.GAppsRemoteURL = gappsSection.GetString("remote_url"); c.GAppsRemoteURL != "" {
		if c.GAppsRemoteHostname = gappsSection.GetString("remote_host"); c.GAppsRemoteHostname == "" {
			return nil, errors.New("you must provide 'gapps.remote_host' along with 'gapps.remote_url'")
		}
	}

	if c.GAppsLocalURL == "" && c.GAppsRemoteURL == "" {
		return nil, errors.New("you must provide either 'gapps.local_url' or 'gapps.remote_url'")
	}

	ghSection := viper.Sub("github")
	if c.GithubRepo = ghSection.GetString("repo"); c.GithubRepo == "" {
		return nil, errors.Errorf(emptyErr, "github.repo")
	}
	if c.GithubToken = ghSection.GetString("token"); c.GithubToken == "" {
		return nil, errors.Errorf(emptyErr, "github.token")
	}

	tgSection := viper.Sub("telegram")
	if c.TelegramToken = tgSection.GetString("token"); c.TelegramToken == "" {
		return nil, errors.Errorf(emptyErr, "telegram.token")
	}
	if c.TelegramTimeout = tgSection.GetInt("timeout"); c.TelegramTimeout <= 0 {
		return nil, errors.Errorf(parseErr, "telegram.timeout")
	}

	msgSection := viper.Sub("messages")
	if c.MsgHello = msgSection.GetString("hello"); c.MsgHello == "" {
		return nil, errors.Errorf(emptyErr, "messages.hello")
	}
	if c.MsgHelp = msgSection.GetString("help"); c.MsgHelp == "" {
		return nil, errors.Errorf(emptyErr, "messages.help")
	}

	mirrorSection := msgSection.Sub("mirror")
	if c.MsgMirrorInProgress = mirrorSection.GetString("in_progress"); c.MsgMirrorInProgress == "" {
		return nil, errors.Errorf(emptyErr, "messages.mirror.in_progress")
	}
	if c.MsgMirrorFound = mirrorSection.GetString("found"); c.MsgMirrorFound == "" {
		return nil, errors.Errorf(emptyErr, "messages.mirror.found")
	}
	if c.MsgMirrorNotFound = mirrorSection.GetString("not_found"); c.MsgMirrorNotFound == "" {
		return nil, errors.Errorf(emptyErr, "messages.mirror.not_found")
	}
	if c.MsgMirrorMissing = mirrorSection.GetString("missing"); c.MsgMirrorMissing == "" {
		return nil, errors.Errorf(emptyErr, "messages.mirror.missing")
	}
	if c.MsgMirrorOK = mirrorSection.GetString("ok"); c.MsgMirrorOK == "" {
		return nil, errors.Errorf(emptyErr, "messages.mirror.ok")
	}
	if c.MsgMirrorFail = mirrorSection.GetString("fail"); c.MsgMirrorFail == "" {
		return nil, errors.Errorf(emptyErr, "messages.mirror.fail")
	}

	errSection := msgSection.Sub("errors")
	if c.MsgErrPlatform = errSection.GetString("platform"); c.MsgErrPlatform == "" {
		return nil, errors.Errorf(emptyErr, "messages.errors.platform")
	}
	if c.MsgErrAndroid = errSection.GetString("android"); c.MsgErrAndroid == "" {
		return nil, errors.Errorf(emptyErr, "messages.errors.android")
	}
	if c.MsgErrVariant = errSection.GetString("variant"); c.MsgErrVariant == "" {
		return nil, errors.Errorf(emptyErr, "messages.errors.variant")
	}
	if c.MsgErrDate = errSection.GetString("date"); c.MsgErrDate == "" {
		return nil, errors.Errorf(emptyErr, "messages.errors.date")
	}
	if c.MsgErrMirror = errSection.GetString("mirror"); c.MsgErrMirror == "" {
		return nil, errors.Errorf(emptyErr, "messages.errors.mirror")
	}
	if c.MsgErrUnknown = errSection.GetString("unknown"); c.MsgErrUnknown == "" {
		return nil, errors.Errorf(emptyErr, "messages.errors.unknown")
	}

	return c, nil
}
