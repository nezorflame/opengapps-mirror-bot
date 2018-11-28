package gapps

import (
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

// TimeFormat is used for gapps packages
const TimeFormat = "20060102"

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
