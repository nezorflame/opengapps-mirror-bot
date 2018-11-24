package gapps

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/google/go-github/v19/github"
	"github.com/pkg/errors"
)

const notFoundErr = "package not found"

// Storage describes a package storage
type Storage struct {
	Date  string
	Count int

	packages map[Platform]map[Android]map[Variant]*Package
	mtx      sync.RWMutex
}

// GetPackageStorage creates and fills a new Storage
func GetPackageStorage(client *github.Client) (*Storage, error) {
	storage := &Storage{
		packages: make(map[Platform]map[Android]map[Variant]*Package, len(PlatformValues())),
	}

	for _, platform := range PlatformValues() {
		releases, resp, err := client.Repositories.GetLatestRelease(context.Background(), "opengapps", platform.String())
		if err != nil {
			log.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			log.Fatal(resp.Status)
		}
		resp.Body.Close()

		zipSlice := make([]github.ReleaseAsset, 0, len(releases.Assets))
		md5Slice := make([]github.ReleaseAsset, 0, len(releases.Assets))

		// Sort out zip and MD5's
		for _, asset := range releases.Assets {
			name := asset.GetName()
			if strings.HasSuffix(name, "zip") {
				zipSlice = append(zipSlice, asset)
			}

			if strings.HasSuffix(name, "md5") {
				md5Slice = append(md5Slice, asset)
			}
		}

		// Sort out packages and fill MD5's
		var wg sync.WaitGroup
		wg.Add(len(zipSlice))
		for i := 0; i < len(zipSlice); i++ {
			go func(wg *sync.WaitGroup, i int) {
				defer wg.Done()
				p, err := formPackage(zipSlice[i], md5Slice[i])
				if err != nil {
					log.Printf("Unable to get package: %v", err)
					return
				}
				storage.AddPackage(p)
			}(&wg, i)
		}
		wg.Wait()
	}

	return storage, nil
}

// AddPackage safely adds a new package to the Storage
func (s *Storage) AddPackage(p *Package) {
	s.mtx.Lock()
	if s.packages[p.Platform] == nil {
		s.packages[p.Platform] = make(map[Android]map[Variant]*Package, len(AndroidValues()))
	}
	if s.packages[p.Platform][p.Android] == nil {
		s.packages[p.Platform][p.Android] = make(map[Variant]*Package, len(VariantValues()))
	}
	if _, ok := s.packages[p.Platform][p.Android][p.Variant]; !ok {
		s.Count++
		s.packages[p.Platform][p.Android][p.Variant] = p
	}
	if s.Date == "" {
		s.Date = p.Date
	}
	s.mtx.Unlock()
}

// GetPackage safely gets a new package from the Storage
func (s *Storage) GetPackage(p Platform, a Android, v Variant) (*Package, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	if s.packages[p] == nil || s.packages[p][a] == nil {
		return nil, errors.New(notFoundErr)
	}

	result, ok := s.packages[p][a][v]
	if !ok {
		return nil, errors.New(notFoundErr)
	}

	return result, nil
}

// Clear cleanes the Storage to be used as new later
func (s *Storage) Clear() {
	s.mtx.Lock()
	s.packages = nil
	s.Count = 0
	s.Date = ""
	s.mtx.Unlock()
}
