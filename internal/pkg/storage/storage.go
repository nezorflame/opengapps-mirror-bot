package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/google/go-github/v37/github"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/nezorflame/opengapps-mirror-bot/internal/pkg/db"
	"github.com/nezorflame/opengapps-mirror-bot/pkg/gapps"
	"github.com/nezorflame/opengapps-mirror-bot/pkg/net"
)

// CurrentStorageKey is used as GlobalStorage key for the current package
const CurrentStorageKey = "current"

// Storage describes a package storage
type Storage struct {
	Date     string                                                          `json:"date"`
	Count    int                                                             `json:"count"`
	Packages map[gapps.Platform]map[gapps.Android]map[gapps.Variant]*Package `json:"packages"`
	cache    *db.DB
	mtx      sync.RWMutex
}

// GetPackageStorage creates and fills a new Storage
func GetPackageStorage(ctx context.Context, ghClient *github.Client, dq *net.DownloadQueue, cfg *viper.Viper, releaseTag string) (*Storage, error) {
	releases, err := getAllReleasesByTag(ctx, ghClient, cfg.GetString("github.repo"), releaseTag)
	if err != nil {
		return nil, fmt.Errorf("unable to get latest releases from Github: %w", err)
	}

	storage := &Storage{Packages: make(map[gapps.Platform]map[gapps.Android]map[gapps.Variant]*Package, len(releases))}
	for _, release := range releases {
		zipSlice := make([]*github.ReleaseAsset, 0, len(release.Assets))
		md5Slice := make([]*github.ReleaseAsset, 0, len(release.Assets))

		// Sort out zip and MD5's
		for _, asset := range release.Assets {
			if asset == nil {
				continue
			}

			name := asset.GetName()
			if strings.HasSuffix(name, "zip") {
				zipSlice = append(zipSlice, asset)
			}

			if strings.HasSuffix(name, "md5") {
				md5Slice = append(md5Slice, asset)
			}
		}

		// Sort out Packages and fill MD5's
		var wg sync.WaitGroup
		wg.Add(len(zipSlice))
		for i := 0; i < len(zipSlice); i++ {
			go func(wg *sync.WaitGroup, i int) {
				defer wg.Done()
				p, err := formPackage(dq, cfg, zipSlice[i], md5Slice[i])
				if err != nil {
					log.Errorf("Unable to form package: %v", err)
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
	if s.Packages[p.Platform] == nil {
		s.Packages[p.Platform] = make(map[gapps.Android]map[gapps.Variant]*Package, len(gapps.AndroidValues()))
	}
	if s.Packages[p.Platform][p.Android] == nil {
		s.Packages[p.Platform][p.Android] = make(map[gapps.Variant]*Package, len(gapps.VariantValues()))
	}
	if _, ok := s.Packages[p.Platform][p.Android][p.Variant]; !ok {
		s.Count++
		s.Packages[p.Platform][p.Android][p.Variant] = p
	}
	if s.Date == "" {
		s.Date = p.Date
	}
	s.mtx.Unlock()
}

// Get safely gets a package from the Storage
func (s *Storage) Get(p gapps.Platform, a gapps.Android, v gapps.Variant) (*Package, bool) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	if s.Packages[p] == nil || s.Packages[p][a] == nil {
		return nil, false
	}

	result, ok := s.Packages[p][a][v]
	return result, ok
}

// Delete safely deletes a package from the Storage (if it's there)
func (s *Storage) Delete(p *Package) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if s.Packages[p.Platform] == nil || s.Packages[p.Platform][p.Android] == nil {
		return
	}

	delete(s.Packages[p.Platform][p.Android], p.Variant)
}

// Save saves the Storage to the cache
func (s *Storage) Save() error {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	body, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("unable to marshal storage %s: %w", s.Date, err)
	}
	if body == nil {
		return fmt.Errorf("storage %s is empty", s.Date)
	}

	if err = s.cache.Put(s.Date, body); err != nil {
		return fmt.Errorf("unable to save storage %s to cache: %w", s.Date, err)
	}
	return nil
}

// GetLatestReleaseDate returns the date for the latest OpenGApps release
func GetLatestReleaseDate(ctx context.Context, ghClient *github.Client, repo string) (string, error) {
	releases, err := getAllReleasesByTag(ctx, ghClient, repo, CurrentStorageKey)
	if err != nil {
		return "", fmt.Errorf("unable to get latest releases from Github: %w", err)
	}

	releaseDates := make([]string, len(releases))
	for i := range releases {
		releaseDates[i] = *releases[i].TagName
	}

	sort.Sort(sort.Reverse(sort.StringSlice(releaseDates)))
	return releaseDates[0], nil
}

func getAllReleasesByTag(ctx context.Context, ghClient *github.Client, repo, tag string) ([]*github.RepositoryRelease, error) {
	var (
		releases = make([]*github.RepositoryRelease, len(gapps.PlatformValues()))
		release  *github.RepositoryRelease
		resp     *github.Response
		count    int
		err      error
	)
	if tag == "" {
		tag = CurrentStorageKey
	}

	for _, platform := range gapps.PlatformValues() {
		if tag == CurrentStorageKey {
			release, resp, err = ghClient.Repositories.GetLatestRelease(ctx, repo, platform.String())
		} else {
			release, resp, err = ghClient.Repositories.GetReleaseByTag(ctx, repo, platform.String(), tag)
		}
		if err != nil {
			log.Errorf("Unable to get release from Github: %v", err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			log.Errorf("Unable to get release from Github with bad response: %s", resp.Status)
			continue
		}
		if release == nil {
			log.Error("Unable to get release from Github with bad response: release is nil")
			continue
		}
		releases[count] = release
		count++
	}
	if count == 0 {
		return nil, errors.New("no releases available")
	}
	return releases[:count-1], nil
}
