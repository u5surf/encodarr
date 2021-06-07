package library

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/BrenekH/encodarr/controller"
	"github.com/google/uuid"
)

func NewManager(logger controller.Logger, ds controller.LibraryManagerDataStorer, metadataReader MetadataReader, commandDecider CommandDecider) Manager {
	return Manager{
		logger:         logger,
		ds:             ds,
		metadataReader: metadataReader,
		commandDecider: commandDecider,
		videoFileser:   defaultVideoFileser{},

		lastCheckedTimes:   make(map[int]time.Time),
		workerCompletedMap: make(map[int]bool),
	}
}

type Manager struct {
	logger         controller.Logger
	ds             controller.LibraryManagerDataStorer
	metadataReader MetadataReader
	commandDecider CommandDecider
	videoFileser   videoFileser

	// lastCheckedTimes is a map of Library ids and the last time that they were checked.
	lastCheckedTimes map[int]time.Time

	// workerCompletedMap is a map of Library ids and a boolean to indicate whether the goroutine that was spawned is finished
	workerCompletedMap map[int]bool
}

func (m *Manager) Start(ctx *context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			if controller.IsContextFinished(ctx) {
				return
			}

			// Check all Libraries for required scans
			allLibraries, err := m.ds.Libraries()
			if err != nil {
				m.logger.Error("%v", err)
				time.Sleep(time.Second)
				continue
			}

			for _, lib := range allLibraries {
				t, ok := m.lastCheckedTimes[lib.ID]
				if !ok {
					m.lastCheckedTimes[lib.ID] = time.Unix(0, 0)
					t = m.lastCheckedTimes[lib.ID]
				}

				previousWorkerFinished, ok := m.workerCompletedMap[lib.ID]
				if !ok {
					m.workerCompletedMap[lib.ID] = true
					previousWorkerFinished = m.workerCompletedMap[lib.ID]
				}

				if time.Since(t) > lib.FsCheckInterval && previousWorkerFinished {
					m.logger.Debug("Initiating library (ID: %v) update", lib.ID)
					m.lastCheckedTimes[lib.ID] = time.Now()
					m.workerCompletedMap[lib.ID] = false

					wg.Add(1)
					go m.updateLibraryQueue(ctx, wg, lib)
				}
			}
			time.Sleep(time.Second)
		}
	}()
}

func (m *Manager) updateLibraryQueue(ctx *context.Context, wg *sync.WaitGroup, lib controller.Library) {
	defer wg.Done()
	defer func() { m.workerCompletedMap[lib.ID] = true }()

	// Locate video files
	discoveredVideos, err := m.videoFileser.VideoFiles(lib.Folder)
	if err != nil {
		m.logger.Error(err.Error())
		return
	}

	for _, videoFilepath := range discoveredVideos {
		// Check path against Library path masks
		maskedOut := false
		for _, v := range lib.PathMasks {
			if strings.Contains(videoFilepath, v) {
				m.logger.Debug("%v skipped because of a mask (%v)", videoFilepath, v)
				maskedOut = true
				break
			}
		}
		if maskedOut {
			continue
		}

		pathDispatched, err := m.ds.IsPathDispatched(videoFilepath)
		if err != nil {
			m.logger.Error(err.Error())
			continue
		}

		if pathDispatched || lib.Queue.InQueuePath(controller.Job{Path: videoFilepath}) {
			continue
		}

		// Read file metadata from a MetadataReader
		fMetadata, err := m.metadataReader.Read(videoFilepath)
		if err != nil {
			m.logger.Error("Skipping %v because of error: %v", videoFilepath, err)
		}

		// Run a CommandDecider against the metadata to determine what FFMpeg command to run
		commandSlice, err := m.commandDecider.Decide(fMetadata, lib.CommandDeciderSettings)
		if err != nil {
			m.logger.Debug("Skipping %v because CommandDecider returned error: %v", videoFilepath, err)
			continue
		}

		// Save to Library queue
		job := controller.Job{
			UUID:     controller.UUID(uuid.NewString()),
			Path:     videoFilepath,
			Command:  commandSlice,
			Metadata: fMetadata,
		}
		lib.Queue.Push(job)
		m.logger.Info("Added %v to Library %v's queue", videoFilepath, lib.ID)

		m.ds.SaveLibrary(lib)
	}
}

func (m *Manager) ImportCompletedJobs(jobs []controller.Job) { // TODO: Replace controller.Job with a better data type for completed jobs.
	m.logger.Critical("Not implemented")
	// TODO: Implement
}

func (m *Manager) LibrarySettings() (ls []controller.Library) { //? Maybe this would benefit from an error return value. I'm thinking it guard against setting the settings in the UI to an empty slice.
	m.logger.Critical("Not implemented")
	// TODO: Implement

	// Steps
	// * Get every library from DataStorer (m.ds.Libraries())
	// * Log any error
	// * Return slice of controller.Library

	return
}

// TODO: Remove
func (m *Manager) LibraryQueues() (lq []controller.LibraryQueue) {
	m.logger.Critical("Not implemented")
	// TODO: Implement
	return
}

// TODO: Add error return value
func (m *Manager) PopNewJob() (j controller.Job) {
	m.logger.Critical("Not implemented")
	// TODO: Implement

	// Steps
	// * Get every library from DataStorer (m.ds.Libraries())
	// * Sort for priority (descending order)
	// * Loop through sorted slice looking for a job to return

	return
}

// UpdateLibrarySettings loops through each entry in the provided map and applies the new settings
// if the key matches a valid library. However, it will not update the ID and Queue fields.
func (m *Manager) UpdateLibrarySettings(map[int]controller.Library) {
	m.logger.Critical("Not implemented")
	// TODO: Implement
}

type defaultVideoFileser struct{}

func (d defaultVideoFileser) VideoFiles(dir string) ([]string, error) {
	return GetVideoFilesFromDir(dir)
}
