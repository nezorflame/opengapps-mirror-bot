package gapps

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v19/github"
	"github.com/nezorflame/opengapps-mirror-bot/config"
	"github.com/nezorflame/opengapps-mirror-bot/utils"
	"github.com/pkg/errors"
)

const (
	parsingErrText = "parsing error"
	gappsSeparator = "-"
)

// Package describes the OpenGApps package
type Package struct {
	Name      string   `json:"name"`
	Date      string   `json:"date"`
	OriginURL string   `json:"origin_url"`
	LocalURL  string   `json:"local_url"`
	RemoteURL string   `json:"remote_url"`
	MD5       string   `json:"md5"`
	Size      int      `json:"size"`
	Platform  Platform `json:"platform"`
	Android   Android  `json:"android"`
	Variant   Variant  `json:"variant"`
}

// CreateMirror creates a new mirror for the package
func (p *Package) CreateMirror(cfg *config.Config) error {
	if cfg.GAppsLocalURL != "" && p.LocalURL != "" || cfg.GAppsRemoteURL != "" && p.RemoteURL != "" {
		return nil
	}

	// download the file
	filePath, err := utils.DownloadFile(p.OriginURL, p.MD5, 20, p.Size)
	if err != nil {
		return errors.Wrap(err, "unable to read file body")
	}
	log.Printf("Package downloaded to %s", filePath)

	// if we have cfg.GAppsLocalPath set, save the file there
	if cfg.GAppsLocalPath != "" {
		if filePath, err = p.move(filePath, cfg.GAppsLocalPath); err != nil {
			return errors.Wrap(err, "unable to move the file to storage")
		}
		log.Printf("Package moved to to %s", filePath)

		// if we have cfg.GAppsLocalURL set, provide the local server URL
		if cfg.GAppsLocalURL != "" {
			relPath := strings.TrimPrefix(filePath, cfg.GAppsLocalPath)
			p.LocalURL = fmt.Sprintf(cfg.GAppsLocalURL, relPath)
			log.Printf("Local URL is %s", p.LocalURL)
		}
	} else {
		// delete the file in the end otherwise
		log.Println("Temp file will be deleted")
		defer os.Remove(filePath)
	}

	// if we have cfg.GAppsRemoteURL set, send the file to remote URL
	if cfg.GAppsRemoteURL != "" {
		tmpFile, err := os.Open(filePath)
		if err != nil {
			return errors.Wrap(err, "unable to create temp file")
		}
		defer tmpFile.Close()

		req, err := http.NewRequest(http.MethodPut, fmt.Sprintf(cfg.GAppsRemoteURL, p.Name), tmpFile)
		if err != nil {
			return errors.Wrap(err, "unable to create upload request")
		}
		req.Header.Set("Content-Type", "application/zip")
		req.Header.Set("Max-Days", "7")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return errors.Wrap(err, "unable to make upload request")
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return errors.Errorf("unable to make upload request: %v", resp.Status)
		}

		result, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return errors.Wrap(err, "unable to read mirror response body")
		}

		p.RemoteURL = string(result)
		log.Printf("File uploaded, remote URL is %s", p.RemoteURL)
	}

	return nil
}

func (p *Package) move(origin, destFolder string) (string, error) {
	path := fmt.Sprintf("%s/%s/%s", destFolder, p.Platform, p.Date)
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", errors.Wrap(err, "unable to create folder")
	}

	path += "/" + p.Name
	if err := os.Rename(origin, path); err != nil {
		return "", errors.Wrap(err, "unable to move file")
	}

	if err := os.Chmod(path, 0755); err != nil {
		return "", errors.Wrap(err, "unable to set file permissions")
	}

	return path, nil
}

// ParsePackageParts helps to parse package info args into proper parts
func ParsePackageParts(args []string) (Platform, Android, Variant, error) {
	if len(args) != 3 {
		return 0, 0, 0, errors.Errorf("bad number of arguments: want 4, got %d", len(args))
	}

	platform, err := PlatformString(args[0])
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, parsingErrText)
	}

	android, err := AndroidString(args[1])
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, parsingErrText)
	}

	variant, err := VariantString(args[2])
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, parsingErrText)
	}

	return platform, android, variant, nil
}

func formPackage(cfg *config.Config, zipAsset, md5Asset github.ReleaseAsset) (*Package, error) {
	md5sum, err := getMD5(md5Asset.GetBrowserDownloadURL())
	if err != nil {
		return nil, errors.Wrap(err, "unable to download md5")
	}

	p, err := parseAsset(cfg, zipAsset, md5sum)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create package")
	}

	return p, nil
}

func getMD5(url string) (string, error) {
	filePath, err := utils.DownloadSingle(url)
	if err != nil {
		return "", errors.Wrap(err, "unable to download MD5 file")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", errors.Wrap(err, "unable to open MD5 file")
	}
	defer file.Close()

	result, err := ioutil.ReadAll(file)
	if err != nil {
		return "", errors.Wrap(err, "unable to read MD5 file")
	}

	return strings.Split(string(result), "  ")[0], nil
}

// Package name format is as follows:
// open_gapps-Platform-Android-Variant-Date.zip
func parseAsset(cfg *config.Config, asset github.ReleaseAsset, md5Sum string) (*Package, error) {
	name := asset.GetName()
	parts := strings.Split(strings.TrimPrefix(name, cfg.GAppsPrefix+gappsSeparator), ".")
	if len(parts) != 3 {
		return nil, errors.Errorf("incorrect package name: %s", name)
	}

	path, ext := parts[0]+parts[1], parts[2]
	if ext != "zip" {
		return nil, errors.Errorf("incorrect package extension: %s", ext)
	}

	parts = strings.Split(path, gappsSeparator)
	if len(parts) != 4 {
		return nil, errors.Errorf("incorrect package name: %s", name)
	}

	platform, android, variant, err := ParsePackageParts(parts[:3])
	if err != nil {
		return nil, err
	}

	if _, err = time.Parse(cfg.GAppsTimeFormat, parts[3]); err != nil {
		return nil, errors.Wrap(err, "unable to parse time")
	}

	return &Package{
		Name:      name,
		Date:      parts[3],
		OriginURL: asset.GetBrowserDownloadURL(),
		MD5:       md5Sum,
		Size:      asset.GetSize(),
		Platform:  platform,
		Android:   android,
		Variant:   variant,
	}, nil
}
