package utils

import (
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/pkg/errors"
)

// DownloadSingle downloads a file from URL in single thread
func DownloadSingle(url string) (string, error) {
	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return "", errors.Wrap(err, "unable to make GET request")
	}
	defer resp.Body.Close()

	tmpFile, err := createTmpFile(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "unable to create result file")
	}

	return tmpFile.Name(), nil
}

// DownloadFile gets the file from URL in multiple threads
func DownloadFile(url, md5sum string, limit, size int) (string, error) {
	var (
		result string
		err    error
	)

	switch {
	case size > 0:
		if result, err = downloadMulti(url, size, limit); err != nil {
			return "", errors.New("unable to download the file")
		}
	case size == 0:
		if result, err = DownloadSingle(url); err != nil {
			return "", errors.New("unable to download the file")
		}
	default:
		return "", errors.New("file size must be more than 0")
	}

	if md5sum != "" {
		if check, err := checkMD5(result, md5sum); err != nil {
			return "", errors.Wrap(err, "unable to check MD5 checksum")
		} else if !check {
			return "", errors.New("checksum mismatch")
		}
	}

	return result, nil
}

func downloadMulti(url string, size, limit int) (string, error) {
	var wg sync.WaitGroup
	wg.Add(limit)
	lenSub, diff := size/limit, size%limit
	tmpFileNames := make([]string, limit)
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

			tmpFile, err := createTmpFile(resp.Body)
			if err != nil {
				log.Println(err)
				return
			}

			tmpFileNames[i] = tmpFile.Name()
			tmpFile.Close()
			wg.Done()
		}(min, max, i)
	}
	wg.Wait()

	tmpFileName, err := joinFiles(tmpFileNames)
	if err != nil {
		return "", errors.Wrap(err, "unable to create result file")
	}

	return tmpFileName, nil
}

func createTmpFile(content io.Reader) (*os.File, error) {
	file, err := ioutil.TempFile("", "*")
	if err != nil {
		return nil, errors.Wrap(err, "unable to create file")
	}

	if content != nil {
		if _, err = io.Copy(file, content); err != nil {
			file.Close()
			return nil, errors.Wrap(err, "unable to write file content")
		}
	}
	return file, nil
}

func joinFiles(filepaths []string) (string, error) {
	if len(filepaths) <= 0 {
		return "", errors.New("nothing to merge")
	}

	if len(filepaths) == 1 {
		return filepaths[0], nil
	}

	dest, err := os.OpenFile(filepaths[0], os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return "", errors.Wrap(err, "unable to open destination file")
	}
	defer dest.Close()

	var source *os.File
	for i := 1; i < len(filepaths); i++ {
		if source, err = os.Open(filepaths[i]); err != nil {
			return "", errors.Wrap(err, "unable to open destination file")
		}
		_, err := io.Copy(dest, source)
		_ = source.Close()
		_ = os.Remove(filepaths[i])
		if err != nil {
			return "", errors.Wrapf(err, "unable to append source file %d to destination", i)
		}
	}
	return filepaths[0], nil
}

func checkMD5(path, md5sum string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		log.Printf("Unable to check MD5: %v", err)
		return false, errors.Wrap(err, "unable to open the file")
	}
	defer file.Close()

	content, err := ioutil.ReadAll(file)
	if err != nil {
		return false, errors.Wrap(err, "unable to read the file")
	}

	result := fmt.Sprintf("%x", md5.Sum(content))
	return result == md5sum, nil
}
