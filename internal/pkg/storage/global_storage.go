package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/nezorflame/opengapps-mirror-bot/internal/pkg/db"
	"github.com/nezorflame/opengapps-mirror-bot/pkg/net"

	"github.com/google/go-github/v29/github"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// GlobalStorage stores all the available storages
type GlobalStorage struct {
	storages map[string]*Storage
	cache    *db.DB
	mtx      sync.RWMutex
}

// NewGlobalStorage creates a new GlobalStorage instance
func NewGlobalStorage(cache *db.DB) *GlobalStorage {
	return &GlobalStorage{
		storages: make(map[string]*Storage),
		cache:    cache,
	}
}

// AddLatestStorage adds the latest Storage to the storages
func (gs *GlobalStorage) AddLatestStorage(ctx context.Context, ghClient *github.Client, dq *net.DownloadQueue, cfg *viper.Viper) error {
	releaseDate, err := GetLatestReleaseDate(ctx, ghClient, cfg.GetString("github.repo"))
	if err != nil {
		return fmt.Errorf("unable to get latest release date: %w", err)
	}
	logger := log.WithField("release_date", releaseDate)
	logger.Debugf("Got the newest release date")

	// check if the current package is in cache and is up-to-date
	// get it if it's not
	s, ok := gs.Get(releaseDate)
	if !ok {
		logger.Info("Storage not found, creating a new one")
		if s, err = GetPackageStorage(ctx, ghClient, dq, cfg, releaseDate); err != nil {
			return fmt.Errorf("unable to get current package storage: %w", err)
		}
		logger.Debug("Saving the storage")
		gs.Add(s.Date, s)
		if err = s.Save(); err != nil {
			return fmt.Errorf("unable to save new storage: %w", err)
		}
		logger.Debug("Storage added successfully")
	}

	logger.Debug("Setting storage as current")
	gs.Add(CurrentStorageKey, s)
	return nil
}

// Add safely adds a new Storage to the storages
func (gs *GlobalStorage) Add(date string, s *Storage) {
	gs.mtx.Lock()
	if date == "" {
		date = CurrentStorageKey
	}
	if s.cache == nil {
		s.cache = gs.cache
	}
	gs.storages[date] = s
	gs.mtx.Unlock()
}

// Get safely gets a Storage from the storages
func (gs *GlobalStorage) Get(date string) (*Storage, bool) {
	gs.mtx.RLock()
	defer gs.mtx.RUnlock()
	s, ok := gs.storages[date]
	if s != nil && s.cache == nil {
		s.cache = gs.cache
	}
	return s, ok
}

// Save saves the GlobalStorage to the cache
func (gs *GlobalStorage) Save() {
	gs.mtx.RLock()
	defer gs.mtx.RUnlock()
	for k, s := range gs.storages {
		if k == CurrentStorageKey {
			continue
		}
		if err := s.Save(); err != nil {
			log.Errorf("Unable to save storage %s: %v", k, err)
		}
	}
}

// Load loads the GlobalStorage from the cache
func (gs *GlobalStorage) Load() error {
	// check the cache first
	cachedStorageList, err := gs.cache.Keys()
	if err != nil {
		return fmt.Errorf("unable to load storage list from cache: %w", err)
	}
	log.Debug("Got the release keys: ", cachedStorageList)

	s := &Storage{}
	var sBody []byte
	for _, k := range cachedStorageList {
		if sBody, err = gs.cache.Get(k); err != nil {
			log.Warnf("Unable to get storage from cache for package '%s': %v", k, err)
			continue
		}

		if err = json.Unmarshal(sBody, s); err != nil {
			log.Warnf("Unable to unmarshal storage from cache for package '%s': %v", k, err)
			continue
		}

		gs.Add(k, s)
	}

	if err != nil {
		return fmt.Errorf("unable to load one of the storages: %w", err)
	}
	return nil
}
