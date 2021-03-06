package net

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"sync"

	log "github.com/sirupsen/logrus"
)

// DownloadQueue is used to limit download process
type DownloadQueue struct {
	tokens chan struct{}
}

// NewQueue creates a new instance of DownloadQueue
func NewQueue(maxCount int) *DownloadQueue {
	return &DownloadQueue{
		tokens: make(chan struct{}, maxCount),
	}
}

// AddSingle gets a file from URL in single thread
func (dq *DownloadQueue) AddSingle(url string) (string, error) {
	dq.acquire()
	defer dq.release()

	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("unable to make GET request: %w", err)
	}
	defer resp.Body.Close()

	tmpFile, err := createTmpFile(resp.Body)
	if err != nil {
		return "", fmt.Errorf("unable to create result file: %w", err)
	}

	return tmpFile.Name(), nil
}

// AddMultiple gets the file from URL in multiple threads
func (dq *DownloadQueue) AddMultiple(url, md5sum string, limit, size int) (string, error) {
	var (
		result string
		err    error
	)

	switch {
	case size > 0:
		if result, err = dq.multi(url, size, limit); err != nil {
			return "", errors.New("unable to download the file")
		}
	case size == 0:
		if result, err = dq.AddSingle(url); err != nil {
			return "", errors.New("unable to download the file")
		}
	default:
		return "", errors.New("file size must be more than 0")
	}

	if md5sum != "" {
		if check, err := checkMD5(result, md5sum); err != nil {
			return "", fmt.Errorf("unable to check MD5 checksum: %w", err)
		} else if !check {
			return "", errors.New("checksum mismatch")
		}
	}

	return result, nil
}

func (dq *DownloadQueue) multi(url string, size, limit int) (string, error) {
	dq.acquire()
	defer dq.release()

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
				log.Errorf("Unable to make request: %v", err)
				return
			}
			defer resp.Body.Close()

			tmpFile, err := createTmpFile(resp.Body)
			if err != nil {
				log.Errorf("Unable to make temp file: %v", err)
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
		return "", fmt.Errorf("unable to create result file: %w", err)
	}

	return tmpFileName, nil
}

func (dq *DownloadQueue) acquire() {
	dq.tokens <- struct{}{}
}

func (dq *DownloadQueue) release() {
	<-dq.tokens
}

func createTmpFile(content io.Reader) (*os.File, error) {
	file, err := ioutil.TempFile("", "*")
	if err != nil {
		return nil, fmt.Errorf("unable to create file: %w", err)
	}

	if content != nil {
		if _, err = io.Copy(file, content); err != nil {
			file.Close()
			return nil, fmt.Errorf("unable to write file content: %w", err)
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
		return "", fmt.Errorf("unable to open destination file: %w", err)
	}
	defer dest.Close()

	var source *os.File
	for i := 1; i < len(filepaths); i++ {
		if source, err = os.Open(filepaths[i]); err != nil {
			return "", fmt.Errorf("unable to open destination file: %w", err)
		}
		_, err := io.Copy(dest, source)
		_ = source.Close()
		_ = os.Remove(filepaths[i])
		if err != nil {
			return "", fmt.Errorf("unable to append source file %d to destination: %w", i, err)
		}
	}
	return filepaths[0], nil
}

func checkMD5(path, md5sum string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("unable to open the file: %w", err)
	}
	defer file.Close()

	content, err := ioutil.ReadAll(file)
	if err != nil {
		return false, fmt.Errorf("unable to read the file: %w", err)
	}

	result := fmt.Sprintf("%x", md5.Sum(content))
	return result == md5sum, nil
}
