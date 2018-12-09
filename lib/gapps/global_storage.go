package gapps

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/google/go-github/v19/github"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/nezorflame/opengapps-mirror-bot/lib/config"
	"github.com/nezorflame/opengapps-mirror-bot/lib/db"
	"github.com/nezorflame/opengapps-mirror-bot/lib/utils"
)

// GlobalStorage stores all the available storages
type GlobalStorage struct {
	log      *zap.SugaredLogger
	storages map[string]*Storage
	cache    *db.DB
	mtx      sync.RWMutex
}

// NewGlobalStorage creates a new GlobalStorage instance
func NewGlobalStorage(log *zap.SugaredLogger, cache *db.DB) *GlobalStorage {
	return &GlobalStorage{
		log:      log,
		storages: make(map[string]*Storage),
		cache:    cache,
	}
}

// Init fills the GlobalStorage with the new Storage for the current release
func (gs *GlobalStorage) Init(ctx context.Context, ghClient *github.Client, dq *utils.DownloadQueue, cfg *config.Config) error {
	var err error
	s := &Storage{}

	// check the cache first
	cachedStorageList, err := gs.cache.Keys()
	if err != nil {
		return errors.Wrap(err, "cache error")
	}
	gs.log.Debug("Got the release keys: ", cachedStorageList)

	var sBody []byte
	for _, k := range cachedStorageList {
		if sBody, err = gs.cache.Get(k); err != nil {
			gs.log.Warnf("Unable to get storage from cache for package '%s': %v", k, err)
			continue
		}

		if err = json.Unmarshal(sBody, s); err != nil {
			gs.log.Warnf("Unable to unmarshal storage from cache for package '%s': %v", k, err)
			continue
		}

		gs.Add(k, s)
	}

	releaseDate, err := GetLatestReleaseDate(ctx, gs.log, ghClient, cfg.GithubRepo)
	if err != nil {
		return errors.Wrap(err, "unable to get latest release date")
	}
	gs.log = gs.log.With("release_date", releaseDate)
	gs.log.Debugf("Got the newest release date")

	// check if the current package is in cache and is up-to-date
	// get it if it's not
	s, ok := gs.Get(releaseDate)
	if !ok {
		gs.log.Info("Storage not found, creating a new one")
		if s, err = GetPackageStorage(ctx, gs.log, ghClient, dq, cfg, CurrentStorageKey); err != nil {
			return errors.Wrap(err, "unable to get current package storage")
		}
		gs.log.Debug("Storage created successfully")
		gs.Add(s.Date, s)
	}

	gs.log.Debug("Setting storage as current")
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
			gs.log.Errorf("Unable to save storage %s: %v", k, err)
		}
	}
}
