package pipeline

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tinytui/tinytui/internal/config"
	"github.com/tinytui/tinytui/internal/tinify"
)

type JobStatus string

const (
	StatusPending    JobStatus = "pending"
	StatusProcessing JobStatus = "processing"
	StatusDone       JobStatus = "done"
	StatusFailed     JobStatus = "failed"
	StatusCancelled  JobStatus = "cancelled"
)

type Job struct {
	ID          string // Path as ID?
	FilePath    string
	OriginalSize int64
	CompressedSize int64
	Status      JobStatus
	Error       error
	SavedBytes  int64
	SavedPercent float64
}

type Pipeline struct {
	client     *tinify.Client
	config     *config.Config
	jobs       []*Job
	queue      chan *Job
	jobMutex   sync.RWMutex
	
	workerCount int
	isPaused    bool
	pauseMutex  sync.RWMutex
	pauseCond   *sync.Cond
	
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	
	updates    chan *Job // For TUI to listen
}

func New(cfg *config.Config, apiKey string) *Pipeline {
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pipeline{
		client:      tinify.NewClient(apiKey),
		config:      cfg,
		workerCount: 2, // Default
		queue:       make(chan *Job, 1000),
		ctx:         ctx,
		cancel:      cancel,
		updates:     make(chan *Job, 100),
	}
	p.pauseCond = sync.NewCond(&p.pauseMutex)
	return p
}

func (p *Pipeline) Configure(concurrency int) {
	if concurrency > 4 {
		concurrency = 4
	}
	if concurrency < 1 {
		concurrency = 1
	}
	p.workerCount = concurrency
}

func (p *Pipeline) Start() {
	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

func (p *Pipeline) Stop() {
	p.cancel()
	p.wg.Wait()
	close(p.updates)
}

func (p *Pipeline) AddFiles(paths []string) {
	p.jobMutex.Lock()
	defer p.jobMutex.Unlock()

	for _, path := range paths {
		// Check duplicates?
		exists := false
		for _, j := range p.jobs {
			if j.FilePath == path && j.Status != StatusDone && j.Status != StatusFailed {
				exists = true
				break
			}
		}
		if exists {
			continue
		}

		info, err := os.Stat(path)
		size := int64(0)
		if err == nil {
			size = info.Size()
		}

		job := &Job{
			ID:           path,
			FilePath:     path,
			OriginalSize: size,
			Status:       StatusPending,
		}
		p.jobs = append(p.jobs, job)
		
		// Send to queue
		select {
		case p.queue <- job:
		default:
			// Buffer full, maybe block or expand buffer? 
			// For now let's hope 1000 is enough
			// Or spawn a feeder routine
			go func(j *Job) {
				p.queue <- j
			}(job)
		}
		
		// Notify update
		p.broadcast(job)
	}
}

func (p *Pipeline) Pause() {
	p.pauseMutex.Lock()
	p.isPaused = true
	p.pauseMutex.Unlock()
}

func (p *Pipeline) Resume() {
	p.pauseMutex.Lock()
	p.isPaused = false
	p.pauseCond.Broadcast() // Wake up workers
	p.pauseMutex.Unlock()
}

func (p *Pipeline) TogglePause() bool {
	p.pauseMutex.Lock()
	defer p.pauseMutex.Unlock()
	p.isPaused = !p.isPaused
	if !p.isPaused {
		p.pauseCond.Broadcast()
	}
	return p.isPaused
}

func (p *Pipeline) worker(id int) {
	defer p.wg.Done()

	for {
		// Check pause
		p.pauseMutex.Lock()
		for p.isPaused {
			p.pauseCond.Wait()
		}
		p.pauseMutex.Unlock()

		select {
		case <-p.ctx.Done():
			return
		case job := <-p.queue:
			if job.Status == StatusCancelled {
				continue
			}
			p.process(job)
		}
	}
}

func (p *Pipeline) process(job *Job) {
	job.Status = StatusProcessing
	p.broadcast(job)

	// Open file
	f, err := os.Open(job.FilePath)
	if err != nil {
		job.Error = err
		job.Status = StatusFailed
		p.broadcast(job)
		return
	}
	defer f.Close()

	// Compress
	// Notice: Compress reads Stream.
	// User Requirement: "Always write to temp file then rename."
	
	// Create temp file
	tmpFile, err := os.CreateTemp("", "tiny-*.tmp")
	if err != nil {
		job.Error = err
		job.Status = StatusFailed
		p.broadcast(job)
		return
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName) // Clean up if fails rename, or if we rename it we must not remove it.
	// Actually we should remove it ONLY if we fail before rename.
	// A better pattern:
	success := false
	defer func() {
		if !success {
			os.Remove(tmpName)
		}
	}()

	r, compressedSize, _, err := p.client.Compress(p.ctx, f, filepath.Base(job.FilePath))
	if err != nil {
		tmpFile.Close()
		job.Error = err
		job.Status = StatusFailed
		p.broadcast(job)
		return
	}
	defer r.Close()

	// content is in r. copy to tmpFile
	if _, err := io.Copy(tmpFile, r); err != nil {
		tmpFile.Close()
		job.Error = err
		job.Status = StatusFailed
		p.broadcast(job)
		return
	}
	tmpFile.Close()

	// Rename / Move
	// "Replace original (atomic safe replace)" OR "Output directory"
	// Suffix defaults to .tiny in config?
	
	// CLI/TUI logic might vary? Pipeline should know dest.
	// For "replace", we use os.Rename (atomic on POSIX mostly)
	
	var finalPath string
	if p.config.OutputMode == "replace" {
		// Replace original
		// If suffix is set, we prefer to append invalidating replace?
		// "Replace original (atomic safe replace)" usually means overwriting input file.
		// "Filename suffix (.tiny default)" implies we DON'T overwrite by default?
		
		// Wait, user Requirement 6:
		// "Output Modes: Replace original ... OR Output directory"
		// "Filename suffix (.tiny default)"
		
		// If replace original mode is ON, do we also use suffix?
		// Typically "Replace" implies overwrite. "Suffix" implies new file.
		// Let's assume if config.Suffix is empty, we overwrite. If set, we append to name.
		// But defaulting to .tiny suggests we don't overwrite by default.
		
		// For now implementing: if OutputDir is set, go there.
		// Else, use same dir.
		// If Suffix is set, append it.
		
		if p.config.OutputDir != "" {
			// output dir
			base := filepath.Base(job.FilePath)
			if p.config.Suffix != "" {
				ext := filepath.Ext(base)
				name := strings.TrimSuffix(base, ext)
				base = name + p.config.Suffix + ext // foo.tiny.png
			}
			finalPath = filepath.Join(p.config.OutputDir, base)
		} else {
			// same dir
			if p.config.Suffix != "" {
				ext := filepath.Ext(job.FilePath)
				name := strings.TrimSuffix(job.FilePath, ext)
				finalPath = name + p.config.Suffix + ext
			} else {
				// Overwrite!
				finalPath = job.FilePath
			}
		}
	} else {
		// OutputDir mode (redundant if logic above covers it)
		// Let's stick to logic above.
	}

	// Ensure dir exists if output dir
	if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
		job.Error = err
		job.Status = StatusFailed
		p.broadcast(job)
		return
	}

	if err := os.Rename(tmpName, finalPath); err != nil {
		// copy fallback for cross-device
		if err := copyFile(tmpName, finalPath); err != nil {
			job.Error = err
			job.Status = StatusFailed
			p.broadcast(job)
			return
		}
	}
	success = true

	// Update stats
	job.CompressedSize = compressedSize
	job.SavedBytes = job.OriginalSize - job.CompressedSize
	if job.OriginalSize > 0 {
		job.SavedPercent = float64(job.SavedBytes) / float64(job.OriginalSize) * 100
	}
	job.Status = StatusDone
	p.broadcast(job)
}

func (p *Pipeline) broadcast(job *Job) {
	select {
	case p.updates <- job:
	default:
	}
}

func (p *Pipeline) Updates() <-chan *Job {
	return p.updates
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil { return err }
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil { return err }
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil { return err }
	return nil
}

func (p *Pipeline) Jobs() []*Job {
	p.jobMutex.RLock()
	defer p.jobMutex.RUnlock()
	// Return copy
	res := make([]*Job, len(p.jobs))
	copy(res, p.jobs)
	return res
}
