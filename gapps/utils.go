package gapps

import (
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

const (
	parsingErrText = "parsing error"
	timeFormat     = "20060102"
)

func getMD5(url string) (string, error) {
	body, err := downloadFile(url)
	if err != nil {
		return "", err
	}

	return strings.Split(string(body), "  ")[0], nil
}

func downloadFile(url string) ([]byte, error) {
	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return nil, errors.Wrap(err, "unable to download the file")
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse the response body")
	}
	return body, nil
}

func parsePackageParts(args []string) (Platform, Android, Variant, error) {
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
