package utils

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/pkg/errors"
)

func CreateFile(path string, content []byte, isTemp bool) (string, error) {
	var (
		file *os.File
		err  error
	)

	if isTemp {
		file, err = ioutil.TempFile(path, "*")
	} else {
		file, err = os.Create(path)
	}
	if err != nil {
		return "", errors.Wrap(err, "unable to create file")
	}
	defer file.Close()

	if _, err = file.Write(content); err != nil {
		return "", errors.Wrap(err, "unable to write content to file")
	}

	return file.Name(), nil
}

// Download gets the file from URL in multiple threads
func Download(url string, limit, size int) ([]byte, error) {
	if size == 0 {
		resp, err := http.Head(url)
		if err != nil {
			return nil, errors.Wrap(err, "unable to make HEAD request to the URL")
		}
		defer resp.Body.Close()

		lengthHeader, ok := resp.Header["Content-Length"]
		if !ok {
			// no length info - try simple download with GET request
			return simpleDownload(url)
		}

		length, err := strconv.Atoi(lengthHeader[0])
		if err != nil {
			return nil, errors.Wrap(err, "unable to parse Content-Length")
		}
		size = length
	}
	lenSub, diff := size/limit, size%limit

	var wg sync.WaitGroup
	wg.Add(limit)

	results := make([][]byte, limit)
	for i := 0; i < limit; i++ {
		min, max := lenSub*i, lenSub*(i+1)
		if i == limit-1 {
			max += diff // Add the remaining bytes in the last request
		}

		go func(min, max, i int) {
			req, _ := http.NewRequest(http.MethodGet, url, nil)
			rangeHeader := "bytes=" + strconv.Itoa(min) + "-" + strconv.Itoa(max-1)
			req.Header.Add("Range", rangeHeader)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Println(err)
				return
			}
			defer resp.Body.Close()

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Println(err)
				return
			}

			results[i] = body
			wg.Done()
		}(min, max, i)
	}
	wg.Wait()

	buffer := bytes.Buffer{}
	buffer.Grow(size)
	for _, b := range results {
		_, _ = buffer.Write(b)
	}

	return buffer.Bytes(), nil
}

func simpleDownload(url string) ([]byte, error) {
	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return nil, errors.Wrap(err, "unable to download the file")
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse file body")
	}
	return body, nil
}
