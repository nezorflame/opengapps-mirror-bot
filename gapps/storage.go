package gapps

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/google/go-github/v19/github"
	"github.com/nezorflame/opengapps-mirror-bot/config"
	"github.com/nezorflame/opengapps-mirror-bot/utils"
	"github.com/pkg/errors"
)

// CurrentStorageKey is used as GlobalStorage key for the current package
const CurrentStorageKey = "current"

// Storage describes a package storage
type Storage struct {
	Date  string
	Count int

	packages map[Platform]map[Android]map[Variant]*Package
	mtx      sync.RWMutex
}

// GetPackageStorage creates and fills a new Storage
func GetPackageStorage(ghClient *github.Client, dq *utils.DownloadQueue, cfg *config.Config, releaseTag string) (*Storage, error) {
	var (
		release *github.RepositoryRelease
		resp    *github.Response
		err     error
	)
	storage := &Storage{
		packages: make(map[Platform]map[Android]map[Variant]*Package, len(PlatformValues())),
	}
	if releaseTag == "" {
		releaseTag = CurrentStorageKey
	}

	for _, platform := range PlatformValues() {
		if releaseTag == CurrentStorageKey {
			release, resp, err = ghClient.Repositories.GetLatestRelease(context.Background(), cfg.GithubRepo, platform.String())
		} else {
			release, resp, err = ghClient.Repositories.GetReleaseByTag(context.Background(), cfg.GithubRepo, platform.String(), releaseTag)
		}

		if err != nil {
			log.Println(err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			log.Println(resp.Status)
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		zipSlice := make([]github.ReleaseAsset, 0, len(release.Assets))
		md5Slice := make([]github.ReleaseAsset, 0, len(release.Assets))

		// Sort out zip and MD5's
		for _, asset := range release.Assets {
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
				p, err := formPackage(dq, cfg, zipSlice[i], md5Slice[i])
				if err != nil {
					log.Printf("Unable to get package: %v", err)
					return
				}
				storage.Add(p)
			}(&wg, i)
		}
		wg.Wait()
	}

	return storage, nil
}

// Add safely adds a new package to the Storage
func (s *Storage) Add(p *Package) {
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

// Get safely gets a package from the Storage
func (s *Storage) Get(p Platform, a Android, v Variant) (*Package, bool) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	if s.packages[p] == nil || s.packages[p][a] == nil {
		return nil, false
	}

	result, ok := s.packages[p][a][v]
	return result, ok
}

// Delete safely deletes a package from the Storage (if it's there)
func (s *Storage) Delete(p *Package) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if s.packages[p.Platform] == nil || s.packages[p.Platform][p.Android] == nil {
		return
	}

	if _, ok := s.packages[p.Platform][p.Android][p.Variant]; ok {
		delete(s.packages[p.Platform][p.Android], p.Variant)
	}
}

// Clear cleanes the Storage to be used as new later
func (s *Storage) Clear() {
	s.mtx.Lock()
	s.packages = nil
	s.Count = 0
	s.Date = ""
	s.mtx.Unlock()
}

// GlobalStorage stores all the available storages
type GlobalStorage struct {
	storages map[string]*Storage
	mtx      sync.RWMutex
}

// NewGlobalStorage creates a new GlobalStorage instance
func NewGlobalStorage() *GlobalStorage {
	return &GlobalStorage{
		storages: make(map[string]*Storage),
	}
}

// Init fills the GlobalStorage with the new Storage for the current release
func (gs *GlobalStorage) Init(ghClient *github.Client, dq *utils.DownloadQueue, cfg *config.Config) error {
	gs.Clear()
	s, err := GetPackageStorage(ghClient, dq, cfg, CurrentStorageKey)
	if err != nil {
		return errors.Wrap(err, "unable to get current package storage")
	}

	gs.Add(CurrentStorageKey, s)
	gs.Add(s.Date, s)
	return nil
}

// Add safely adds a new Storage to the storages
func (gs *GlobalStorage) Add(date string, s *Storage) {
	gs.mtx.Lock()
	if date == "" {
		date = CurrentStorageKey
	}
	gs.storages[date] = s
	gs.mtx.Unlock()
}

// Get safely gets a Storage from the storages
func (gs *GlobalStorage) Get(date string) (*Storage, bool) {
	gs.mtx.RLock()
	defer gs.mtx.RUnlock()
	s, ok := gs.storages[date]
	return s, ok
}

// Clear cleanes the GlobalStorage to be used as new later
func (gs *GlobalStorage) Clear() {
	gs.mtx.Lock()
	for k := range gs.storages {
		delete(gs.storages, k)
	}
	gs.mtx.Unlock()
}
