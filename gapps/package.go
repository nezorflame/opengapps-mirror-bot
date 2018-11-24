package gapps

import (
	"strings"
	"time"

	"github.com/google/go-github/v19/github"
	"github.com/pkg/errors"
)

const (
	gappsPrefix    = "open_gapps"
	gappsSeparator = "-"
)

// Package describes the OpenGApps package
type Package struct {
	Name     string   `json:"name"`
	Date     string   `json:"date"`
	URL      string   `json:"url"`
	MD5      string   `json:"md5"`
	Size     int      `json:"size"`
	Platform Platform `json:"platform"`
	Android  Android  `json:"android"`
	Variant  Variant  `json:"variant"`
}

func formPackage(zipAsset, md5Asset github.ReleaseAsset) (*Package, error) {
	md5sum, err := getMD5(md5Asset.GetBrowserDownloadURL())
	if err != nil {
		return nil, errors.Wrap(err, "unable to download md5")
	}

	p, err := parseAsset(zipAsset, md5sum)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create package")
	}

	return p, nil
}

// Package name format is as follows:
// open_gapps-Platform-Android-Variant-Date.zip
func parseAsset(asset github.ReleaseAsset, md5Sum string) (*Package, error) {
	name := asset.GetName()
	parts := strings.Split(strings.TrimPrefix(name, gappsPrefix+gappsSeparator), ".")
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

	platform, android, variant, err := parsePackageParts(parts[:3])
	if err != nil {
		return nil, err
	}

	if _, err = time.Parse(timeFormat, parts[3]); err != nil {
		return nil, errors.Wrap(err, "unable to parse time")
	}

	return &Package{
		Name:     name,
		Date:     parts[3],
		URL:      asset.GetBrowserDownloadURL(),
		MD5:      md5Sum,
		Size:     asset.GetSize(),
		Platform: platform,
		Android:  android,
		Variant:  variant,
	}, nil
}
