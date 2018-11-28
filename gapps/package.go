package gapps

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v19/github"
	"github.com/pkg/errors"
)

const (
	parsingErrText = "parsing error"
	gappsPrefix    = "open_gapps"
	gappsSeparator = "-"
	uploadURL      = "https://transfer.sh/%s"
)

// Package describes the OpenGApps package
type Package struct {
	Name      string   `json:"name"`
	Date      string   `json:"date"`
	OriginURL string   `json:"origin_url"`
	MirrorURL string   `json:"mirror_url"`
	MD5       string   `json:"md5"`
	Size      int      `json:"size"`
	Platform  Platform `json:"platform"`
	Android   Android  `json:"android"`
	Variant   Variant  `json:"variant"`
}

// CreateMirror creates a new mirror for the package
func (p *Package) CreateMirror() error {
	if p.MirrorURL != "" {
		return nil
	}

	// download the file
	resp, err := http.DefaultClient.Get(p.OriginURL)
	if err != nil {
		return errors.Wrap(err, "unable to download the file")
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "unable to read file body")
	}

	// create temp file to store the contents
	tmpfile, err := ioutil.TempFile("", "*")
	if err != nil {
		return errors.Wrap(err, "unable to create temp file")
	}
	defer func() {
		_ = tmpfile.Close()
		_ = os.Remove(tmpfile.Name())
	}()

	if _, err = io.Copy(tmpfile, resp.Body); err != nil {
		return errors.Wrap(err, "unable to write body to temp file")
	}

	fileType := http.DetectContentType(body)
	resp.Body.Close()

	// send the form
	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf(uploadURL, p.Name), tmpfile)
	if err != nil {
		return errors.Wrap(err, "unable to create upload request")
	}
	req.Header.Set("Content-Type", fileType)
	req.Header.Set("Max-Days", "7")

	resp, err = http.DefaultClient.Do(req)
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

	p.MirrorURL = string(result)
	return nil
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

	platform, android, variant, err := ParsePackageParts(parts[:3])
	if err != nil {
		return nil, err
	}

	if _, err = time.Parse(TimeFormat, parts[3]); err != nil {
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
