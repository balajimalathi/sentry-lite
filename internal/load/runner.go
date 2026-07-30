package load

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type RunState int

const (
	StateIdle RunState = iota
	StateRunning
	StatePaused
	StateDone
	StateCrashProbe
)

type Runner struct {
	cfg      Config
	client   *Client
	counters *Counters
	gen      *Gen

	mu         sync.Mutex
	state      RunState
	cancel     context.CancelFunc
	cronToken  string
	alertsSeed bool
	startedAt  time.Time
	doneCh     chan struct{}

	crashProbe bool
}

func NewRunner(cfg Config) *Runner {
	return &Runner{
		cfg:      cfg,
		client:   NewClient(cfg),
		counters: &Counters{},
		gen:      NewGen(time.Now().UnixNano()),
		doneCh:   make(chan struct{}),
	}
}

func (r *Runner) Counters() *Counters { return r.counters }

func (r *Runner) State() RunState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

func (r *Runner) StartedAt() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.startedAt
}

func (r *Runner) Done() <-chan struct{} { return r.doneCh }

func (r *Runner) Prepare(ctx context.Context) error {
	if !r.alertsSeed {
		if err := r.client.SeedAlerts(ctx); err != nil {
			// non-fatal: rules may already exist or project missing
		}
		r.alertsSeed = true
	}
	token, err := r.client.EnsureCron(ctx)
	if err != nil {
		// non-fatal: disable cron traffic if monitor cannot be created
		r.cronToken = ""
		r.cfg.Mix[CatCron] = 0
		return nil
	}
	r.cronToken = token
	return nil
}

func (r *Runner) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == StateRunning || r.state == StateCrashProbe {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.state = StateRunning
	r.crashProbe = false
	r.startedAt = time.Now()
	r.counters = &Counters{}
	r.doneCh = make(chan struct{})
	go r.run(ctx)
	return nil
}

func (r *Runner) StartCrashProbe() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		r.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.state = StateCrashProbe
	r.crashProbe = true
	r.startedAt = time.Now()
	r.counters = &Counters{}
	r.doneCh = make(chan struct{})
	go r.runCrashProbe(ctx)
	return nil
}

func (r *Runner) Pause() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == StateRunning {
		r.state = StatePaused
	}
}

func (r *Runner) Resume() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == StatePaused {
		r.state = StateRunning
	}
}

func (r *Runner) Stop() {
	r.mu.Lock()
	if r.cancel != nil {
		r.cancel()
	}
	r.mu.Unlock()
}

func (r *Runner) finish() {
	r.mu.Lock()
	if r.state != StateDone {
		r.state = StateDone
	}
	r.mu.Unlock()
	select {
	case <-r.doneCh:
	default:
		close(r.doneCh)
	}
}

func (r *Runner) run(ctx context.Context) {
	defer r.finish()
	if err := r.Prepare(ctx); err != nil {
		return
	}

	var sent atomic.Int64
	limiter := newRateLimiter(r.cfg.RPS)
	peak := newPeakController(r.cfg)

	sem := make(chan struct{}, r.cfg.Workers)
	var wg sync.WaitGroup

dispatch:
	for {
		select {
		case <-ctx.Done():
			break dispatch
		default:
		}

		r.mu.Lock()
		st := r.state
		r.mu.Unlock()
		if st == StatePaused {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if st == StateDone {
			break
		}

		if sent.Load() >= r.cfg.Total {
			break
		}

		var wait time.Duration
		if r.crashProbe {
			wait = 0
		} else if r.cfg.Mode == ModeStress {
			wait = limiter.Wait()
		} else {
			wait = peak.Wait()
		}
		if wait > 0 {
			select {
			case <-ctx.Done():
				break dispatch
			case <-time.After(wait):
			}
		}

		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break dispatch
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			r.dispatchOne(ctx, &sent)
		}()
	}
	wg.Wait()
}

func (r *Runner) runCrashProbe(ctx context.Context) {
	defer r.finish()
	if err := r.Prepare(ctx); err != nil {
		return
	}
	sem := make(chan struct{}, r.cfg.Workers*4)
	var wg sync.WaitGroup
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		default:
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			var n atomic.Int64
			r.dispatchOne(ctx, &n)
		}()
	}
}

func (r *Runner) dispatchOne(ctx context.Context, sent *atomic.Int64) {
	if r.State() != StateCrashProbe {
		if sent.Add(1) > r.cfg.Total {
			sent.Add(-1)
			return
		}
	} else {
		sent.Add(1)
	}
	r.counters.InFlight.Add(1)
	defer r.counters.InFlight.Add(-1)

	cat := r.gen.PickCategory(r.cfg.Mix)
	var res SendResult

	if cat == CatCron {
		if r.cronToken == "" {
			return
		}
		status := "ok"
		if r.gen.rng.Intn(5) == 0 {
			status = "error"
		}
		res = r.client.CronCheckIn(ctx, r.cronToken, status)
	} else {
		body, err := r.gen.Build(cat)
		if err == errSkipIngest {
			res = r.client.CronCheckIn(ctx, r.cronToken, "ok")
		} else if err != nil {
			res = SendResult{Err: err}
		} else {
			res = r.client.SendStore(ctx, body)
		}
	}

	r.counters.Record(cat, res)
}

type rateLimiter struct {
	interval time.Duration
	next     time.Time
}

func newRateLimiter(rps int) *rateLimiter {
	if rps < 1 {
		rps = 1
	}
	return &rateLimiter{interval: time.Second / time.Duration(rps)}
}

func (l *rateLimiter) Wait() time.Duration {
	now := time.Now()
	if l.next.IsZero() {
		l.next = now
	}
	if now.Before(l.next) {
		d := l.next.Sub(now)
		l.next = l.next.Add(l.interval)
		return d
	}
	l.next = now.Add(l.interval)
	return 0
}

type peakController struct {
	cfg       Config
	phase     int // 0 idle, 1 burst
	phaseEnd  time.Time
	limiter   *rateLimiter
}

func newPeakController(cfg Config) *peakController {
	p := &peakController{cfg: cfg, phase: 0}
	p.resetLimiter(cfg.IdleRPS)
	p.phaseEnd = time.Now().Add(time.Duration(cfg.IdleSec) * time.Second)
	return p
}

func (p *peakController) resetLimiter(rps int) {
	if rps < 1 {
		rps = 1
	}
	p.limiter = newRateLimiter(rps)
}

func (p *peakController) Wait() time.Duration {
	now := time.Now()
	if now.After(p.phaseEnd) {
		if p.phase == 0 {
			p.phase = 1
			p.resetLimiter(p.cfg.PeakRPS)
			p.phaseEnd = now.Add(time.Duration(p.cfg.BurstSec) * time.Second)
		} else {
			p.phase = 0
			p.resetLimiter(p.cfg.IdleRPS)
			p.phaseEnd = now.Add(time.Duration(p.cfg.IdleSec) * time.Second)
		}
	}
	return p.limiter.Wait()
}
