package gx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"reflect"
	"slices"
	"sync"
	"syscall"
	"time"
)

// runtimeDefaultShutdownTimeout is the default timeout for Runtime shutdown.
const runtimeDefaultShutdownTimeout = 30 * time.Second

// runtimeState tracks Runtime lifecycle state.
type runtimeState uint8

const (
	runtimeSetup runtimeState = iota
	runtimeInitializing
	runtimeRunning
	runtimeStopped
)

// RunOptions configures a Runtime run.
type RunOptions struct {
	// Logger receives Runtime and Container lifecycle logs.
	// A nil logger disables logging.
	Logger *slog.Logger

	// NoSignals disables Runtime-managed signal handling.
	NoSignals bool

	// Signals replaces the default runtime shutdown signals.
	//
	// A nil or empty slice defaults to os.Interrupt and syscall.SIGTERM.
	Signals []os.Signal

	// ShutdownTimeout bounds the context passed to Container.OnStopping and
	// Container.OnStop hooks. If managed work remains active when it expires,
	// Runtime logs the outstanding work at this interval and continues waiting
	// for it to exit.
	//
	// It does not bound Runtime.Run itself: Runtime still waits for managed
	// tasks to exit after stopping hooks have been invoked.
	ShutdownTimeout time.Duration
}

// runOptions returns the effective RunOptions.
func runOptions(opts []RunOptions) RunOptions {
	options := RunOptions{
		ShutdownTimeout: runtimeDefaultShutdownTimeout,
	}

	if len(opts) == 0 {
		return options
	}

	options.Logger = opts[0].Logger
	options.NoSignals = opts[0].NoSignals
	options.Signals = opts[0].Signals

	if opts[0].ShutdownTimeout > 0 {
		options.ShutdownTimeout = opts[0].ShutdownTimeout
	}

	return options
}

// Runtime manages application containers and lifecycle.
type Runtime struct {
	containers      []*Container
	ctx             context.Context
	logger          *slog.Logger
	mu              sync.Mutex
	runner          *Runner
	shutdownOnce    sync.Once
	shutdownTimeout time.Duration
	startupDone     chan struct{}
	state           runtimeState
	stopErr         error
	store           *store
}

// NewRuntime creates a Runtime.
func NewRuntime() *Runtime {
	return &Runtime{
		store: newStore(),
	}
}

// Attach creates and registers a named Container.
//
// Names must be non-empty and unique. Attach may only be called before Run.
func (r *Runtime) Attach(name string) *Container {
	if name == "" {
		panic("gx: empty container name")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state != runtimeSetup {
		panic("gx: container attached after Runtime.Run: " + name)
	}

	for _, c := range r.containers {
		if c.name == name {
			panic("gx: duplicate container name: " + name)
		}
	}

	c := &Container{
		name:    name,
		runtime: r,
		store:   newStore(),
	}

	r.containers = append(r.containers, c)
	return c
}

// Create attaches a named Container and registers a factory that creates a
// Runtime-global dependency during initialization.
//
// fn may use container context and global dependencies. It may register
// OnStart, OnStopping, and OnStop hooks and tasks. A non-nil error stops
// initialization and is returned from Run. Panics from fn are not recovered.
func (r *Runtime) Create[T any](name string, fn func(*Container) (T, error)) *Runtime {
	if fn == nil {
		panic("gx: nil Runtime.Create factory")
	}

	c := r.Attach(name)
	c.OnInit("create", func(c *Container) error {
		value, err := fn(c)
		if err != nil {
			return err
		}

		r.Set(value)
		return nil
	})

	return r
}

// Get returns a runtime-global dependency.
//
// It panics when no value of T is registered.
func (r *Runtime) Get[T any]() T {
	v, ok := r.GetTry[T]()
	if !ok {
		panic(fmt.Sprintf("gx: runtime has no value for type %s", reflect.TypeFor[T]()))
	}
	return v
}

// GetTry returns a runtime-global dependency, if registered.
func (r *Runtime) GetTry[T any]() (T, bool) {
	v, ok := r.store.get(reflect.TypeFor[T]())
	if !ok {
		var zero T
		return zero, false
	}

	return getValue[T](v), true
}

// Run initializes and starts containers in attachment order, then shuts them
// down in reverse attachment order. It returns the first init/startup/task
// error. If initialization, startup, and tasks succeed, it returns the first
// stop-hook error, if any.
//
// At most one RunOptions value may be supplied.
func (r *Runtime) Run(ctx context.Context, opts ...RunOptions) error {
	if ctx == nil {
		panic("gx: nil Runtime.Run context")
	}
	if len(opts) > 1 {
		panic("gx: Runtime.Run accepts at most one RunOptions value")
	}

	options := runOptions(opts)

	ctx, stop := r.signalContext(ctx, options)
	defer stop()

	var runnerCtx context.Context
	var cancelTasks context.CancelFunc

	r.mu.Lock()

	if r.state != runtimeSetup {
		r.mu.Unlock()
		panic("gx: Runtime.Run called more than once")
	}

	r.ctx, cancelTasks = newRuntimeTaskContext(ctx)
	r.logger = options.Logger
	r.runner, runnerCtx = NewRunner(ctx)
	r.shutdownTimeout = options.ShutdownTimeout
	r.startupDone = make(chan struct{})
	r.state = runtimeInitializing

	containers := slices.Clone(r.containers)

	r.mu.Unlock()

	stopRunnerCancellation := context.AfterFunc(runnerCtx, cancelTasks)
	defer stopRunnerCancellation()
	defer cancelTasks()

	// This runs shutdown as soon as a signal, parent cancellation, task
	// failure, or startup failure cancels r.ctx.
	cancelShutdown := context.AfterFunc(r.ctx, r.shutdown)
	defer cancelShutdown()

	r.start(containers)

	err := r.runner.Wait()
	cancelTasks()

	// Once serializes this with the cancellation-triggered shutdown, whichever runs first.
	r.shutdown()

	r.mu.Lock()
	r.state = runtimeStopped
	r.mu.Unlock()

	// A startup or task failure takes precedence over a stop-hook failure.
	if err != nil {
		return err
	}

	return r.stopErr
}

// Set registers a runtime-global dependency.
//
// Dependencies may be registered before or during Runtime initialization.
// It panics if a value of the same type is already registered.
func (r *Runtime) Set[T any](value T) *Runtime {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state != runtimeSetup && r.state != runtimeInitializing {
		panic("gx: runtime dependency set after Runtime.Run")
	}

	t := reflect.TypeFor[T]()
	if !r.store.set(t, value) {
		panic(fmt.Sprintf("gx: runtime already has value for type %s", t))
	}

	return r
}

// log writes a log message using the Runtime's logger if it is set.
func (r *Runtime) log(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	if r.logger != nil {
		r.logger.LogAttrs(ctx, level, msg, attrs...)
	}
}

// logShutdownTimeout reports managed work that did not exit before the
// Runtime shutdown timeout. It returns whether managed work remains active.
func (r *Runtime) logShutdownTimeout(elapsed time.Duration) bool {
	tasks := r.runningTasks()
	if len(tasks) == 0 {
		return false
	}

	elapsed = elapsed.Truncate(r.shutdownTimeout)

	for _, task := range tasks {
		r.log(
			r.ctx,
			slog.LevelError,
			"runtime shutdown timeout exceeded while waiting for managed work",
			slog.String("timeout", r.shutdownTimeout.String()),
			slog.String("elapsed", elapsed.String()),
			slog.String("container", task.container),
			slog.String("task", task.name),
		)
	}

	return true
}

// shutdown stops containers in reverse attachment order.
func (r *Runtime) shutdown() {
	r.shutdownOnce.Do(func() {
		// Do not stop a container while its synchronous startup hook is
		// still running and may register additional work.
		<-r.startupDone

		base := context.WithoutCancel(r.ctx)
		ctx, cancel := context.WithTimeout(base, r.shutdownTimeout)
		defer cancel()

		watchdogDone := make(chan struct{})
		defer close(watchdogDone)
		go r.watchShutdown(watchdogDone)

		r.mu.Lock()
		containers := slices.Clone(r.containers)
		r.mu.Unlock()

		for _, container := range slices.Backward(containers) {
			if err := container.shutdown(ctx); err != nil && r.stopErr == nil {
				r.stopErr = err
			}
		}
	})
}

// runningTasks returns a snapshot of managed work that has not exited.
func (r *Runtime) runningTasks() []runtimeTask {
	r.mu.Lock()
	containers := slices.Clone(r.containers)
	r.mu.Unlock()

	var tasks []runtimeTask
	for _, container := range containers {
		for _, name := range container.runningTasks() {
			tasks = append(tasks, runtimeTask{container: container.name, name: name})
		}
	}

	return tasks
}

// signalContext adds Runtime-managed signal cancellation to ctx.
func (r *Runtime) signalContext(
	ctx context.Context,
	opts RunOptions,
) (context.Context, context.CancelFunc) {
	if opts.NoSignals {
		return ctx, func() {}
	}

	signals := opts.Signals
	if len(signals) == 0 {
		signals = []os.Signal{os.Interrupt, syscall.SIGTERM}
	}

	return signal.NotifyContext(ctx, signals...)
}

// start initializes then starts containers in attachment order.
func (r *Runtime) start(containers []*Container) {
	defer close(r.startupDone)

	for _, c := range containers {
		if r.ctx.Err() != nil {
			return
		}

		if err := c.init(); err != nil {
			// If cancellation was already caused by a task or parent context,
			// preserve that existing cause rather than turning shutdown into a
			// second startup failure.
			if r.ctx.Err() == nil {
				r.runner.Cancel(err)
			}
			return
		}
	}

	if r.ctx.Err() != nil {
		return
	}

	r.mu.Lock()
	r.state = runtimeRunning
	r.mu.Unlock()

	for _, c := range containers {
		if r.ctx.Err() != nil {
			return
		}

		if err := c.start(); err != nil {
			// If cancellation was already caused by a task or parent context,
			// preserve that existing cause rather than turning shutdown into a
			// second startup failure.
			if r.ctx.Err() == nil {
				r.runner.Cancel(err)
			}
			return
		}
	}
}

// watchShutdown periodically reports managed work that did not exit during
// Runtime shutdown.
func (r *Runtime) watchShutdown(done <-chan struct{}) {
	started := time.Now()
	timer := time.NewTimer(r.shutdownTimeout)
	defer timer.Stop()

	select {
	case <-done:
		return
	case <-timer.C:
	}

	ticker := time.NewTicker(r.shutdownTimeout)
	defer ticker.Stop()

	for {
		if !r.logShutdownTimeout(time.Since(started)) {
			return
		}

		select {
		case <-done:
			return
		case <-ticker.C:
		}
	}
}

// newRuntimeTaskContext returns a task context that preserves parent values and
// deadlines while mirroring parent cancellation as context.Canceled.
func newRuntimeTaskContext(parent context.Context) (context.Context, context.CancelFunc) {
	base := context.WithoutCancel(parent)
	cancelDeadline := func() {}
	if deadline, ok := parent.Deadline(); ok {
		base, cancelDeadline = context.WithDeadline(base, deadline)
	}

	ctx, cancel := context.WithCancel(base)
	stopParentCancellation := context.AfterFunc(parent, func() {
		if errors.Is(parent.Err(), context.Canceled) {
			cancel()
		}
	})

	return ctx, func() {
		stopParentCancellation()
		cancel()
		cancelDeadline()
	}
}

// containerState tracks Container lifecycle state.
type containerState uint8

const (
	containerSetup containerState = iota
	containerInitializing
	containerInitialized
	containerStarting
	containerRunning
	containerStopping
	containerStopped
)

// containerHook is synchronous lifecycle work registered with a Container.
type containerHook struct {
	fn   func(*Container) error
	name string
}

// containerStop is synchronous shutdown work registered with a Container.
type containerStop struct {
	fn   func(context.Context, *Container) error
	name string
}

// containerTask is a task registered with a Container.
type containerTask struct {
	fn   func(context.Context) error
	name string
}

// runtimeTask identifies managed work running during Runtime shutdown.
type runtimeTask struct {
	container string
	name      string
}

// Container is a named child scope of a Runtime.
type Container struct {
	initFns     []containerHook
	mu          sync.Mutex
	name        string
	runtime     *Runtime
	running     []string
	startFns    []containerHook
	state       containerState
	stopFns     []containerStop
	stoppingFns []containerStop
	store       *store
	tasks       []containerTask
	wg          sync.WaitGroup
}

// Ctx is valid from OnInit onward and is canceled when shutdown begins.
func (c *Container) Ctx() context.Context {
	return c.runtime.ctx
}

// Get returns a container-local dependency.
//
// It panics when no value of T is registered.
func (c *Container) Get[T any]() T {
	v, ok := c.GetTry[T]()
	if !ok {
		panic(
			fmt.Sprintf("gx: container %q has no value for type %s", c.name, reflect.TypeFor[T]()),
		)
	}
	return v
}

// GetTry looks up a container-local dependency.
func (c *Container) GetTry[T any]() (T, bool) {
	v, ok := c.store.get(reflect.TypeFor[T]())
	if !ok {
		var zero T
		return zero, false
	}

	return getValue[T](v), true
}

// Global returns a Runtime-global dependency.
//
// It panics when no value of T is registered.
func (c *Container) Global[T any]() T {
	return c.runtime.Get[T]()
}

// GlobalTry returns a Runtime-global dependency, if registered.
func (c *Container) GlobalTry[T any]() (T, bool) {
	return c.runtime.GetTry[T]()
}

// Name returns the container name.
func (c *Container) Name() string {
	return c.name
}

// OnInit registers named synchronous initialization work.
//
// Register hooks during construction. Hooks run in registration order before
// any OnStart hook runs. fn may access dependencies and register OnStart,
// OnStopping, OnStop hooks, or tasks. Tasks registered here start after this
// container's startup hooks complete. name identifies the step in logs.
// Panics from fn are not recovered.
func (c *Container) OnInit(name string, fn func(*Container) error) *Container {
	if name == "" {
		panic("gx: empty container init name")
	}
	if fn == nil {
		panic("gx: nil container init hook")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != containerSetup {
		panic("gx: container init hook registered after initialization: " + c.name)
	}

	c.initFns = append(c.initFns, containerHook{name: name, fn: fn})
	return c
}

// OnStart registers named synchronous startup work.
//
// Register hooks during construction. Hooks run in registration order and may
// also be registered from OnInit. All hooks must succeed before tasks registered
// during construction or OnInit start. fn may access dependencies and register
// tasks. name identifies the step in logs.
// Panics from fn are not recovered.
func (c *Container) OnStart(name string, fn func(*Container) error) *Container {
	if name == "" {
		panic("gx: empty container start name")
	}
	if fn == nil {
		panic("gx: nil container start hook")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != containerSetup && c.state != containerInitializing {
		panic("gx: container start hook registered after initialization: " + c.name)
	}

	c.startFns = append(c.startFns, containerHook{name: name, fn: fn})
	return c
}

// OnStop registers named finalization work.
//
// Register hooks during construction. Hooks run in reverse registration order
// after this container's tasks exit and may also be registered from OnInit.
// fn receives a context bounded by RunOptions.ShutdownTimeout and the container.
// name identifies the step in logs.
// Panics from fn are not recovered.
func (c *Container) OnStop(name string, fn func(context.Context, *Container) error) *Container {
	if name == "" {
		panic("gx: empty container stop name")
	}
	if fn == nil {
		panic("gx: nil container stop hook")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != containerSetup && c.state != containerInitializing {
		panic("gx: container stop hook registered after initialization: " + c.name)
	}

	c.stopFns = append(c.stopFns, containerStop{name: name, fn: fn})
	return c
}

// OnStopping registers named work while shutdown is in progress.
//
// Register hooks during construction. Hooks run in reverse registration order
// and may also be registered from OnInit. fn receives a context bounded by
// RunOptions.ShutdownTimeout and the container. It may unblock managed tasks.
// name identifies the step in logs.
// Panics from fn are not recovered.
func (c *Container) OnStopping(name string, fn func(context.Context, *Container) error) *Container {
	if name == "" {
		panic("gx: empty container stopping name")
	}
	if fn == nil {
		panic("gx: nil container stopping hook")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != containerSetup && c.state != containerInitializing {
		panic("gx: container stopping hook registered after initialization: " + c.name)
	}

	c.stoppingFns = append(c.stoppingFns, containerStop{name: name, fn: fn})
	return c
}

// Run registers a named managed task with the container.
//
// Register tasks during construction, from OnInit, or from OnStart. Tasks
// registered during construction or OnInit start only after all OnStart hooks
// succeed; they do not start if an OnStart hook fails. Tasks registered from
// OnStart launch immediately.
//
// Panics from fn are not recovered.
func (c *Container) Run(name string, fn func(context.Context) error) *Container {
	if name == "" {
		panic("gx: empty container task name")
	}
	if fn == nil {
		panic("gx: nil container task")
	}

	task := containerTask{name: name, fn: fn}

	c.mu.Lock()
	defer c.mu.Unlock()

	switch c.state {
	case containerSetup, containerInitializing:
		c.tasks = append(c.tasks, task)

	case containerStarting:
		// Runtime.Run cannot call runner.Wait until c.start returns.
		// Launch while holding c.mu so startup cannot transition to
		// containerRunning before this task has been registered.
		c.startTaskLocked(task)

	default:
		panic("gx: container task registered after initialization: " + c.name)
	}

	return c
}

// Set registers a container-local dependency.
//
// Dependencies may be registered during construction, OnInit, or OnStart.
// It panics if a value of the same type is already registered.
func (c *Container) Set[T any](value T) *Container {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != containerSetup &&
		c.state != containerInitializing &&
		c.state != containerStarting {
		panic("gx: container dependency set after startup: " + c.name)
	}

	t := reflect.TypeFor[T]()
	if !c.store.set(t, value) {
		panic(fmt.Sprintf("gx: container %q already has value for %s", c.name, t))
	}

	return c
}

// SetGlobal registers a Runtime-global dependency.
//
// It panics after Runtime initialization or when a value of type T is already
// registered.
func (c *Container) SetGlobal[T any](value T) *Container {
	c.runtime.Set(value)
	return c
}

// init is called only by Runtime.Run, in attachment order.
func (c *Container) init() error {
	c.mu.Lock()
	if c.state != containerSetup {
		c.mu.Unlock()
		panic("gx: container initialized more than once: " + c.name)
	}

	c.state = containerInitializing
	initFns := slices.Clone(c.initFns)
	c.mu.Unlock()

	for _, hook := range initFns {
		if c.runtime.ctx.Err() != nil {
			return nil
		}

		c.logStep(c.runtime.ctx, slog.LevelDebug, "container step starting", "init", hook.name)

		if err := hook.fn(c); err != nil {
			c.logStep(
				c.runtime.ctx,
				slog.LevelError,
				"container step error",
				"init",
				hook.name,
				slog.String("err", err.Error()),
			)
			return fmt.Errorf("%s: %s: %w", c.name, hook.name, err)
		}

		c.logStep(c.runtime.ctx, slog.LevelDebug, "container step completed", "init", hook.name)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.runtime.ctx.Err() != nil {
		return nil
	}

	c.state = containerInitialized
	return nil
}

// logStep writes a Container lifecycle-step log message.
func (c *Container) logStep(
	ctx context.Context,
	level slog.Level,
	msg string,
	phase string,
	step string,
	attrs ...slog.Attr,
) {
	attrs = append(
		[]slog.Attr{
			slog.String("container", c.name),
			slog.String("phase", phase),
			slog.String("step", step),
		},
		attrs...,
	)
	c.runtime.log(ctx, level, msg, attrs...)
}

// shutdown is called by Runtime.beginShutdown, in reverse attach order.
func (c *Container) shutdown(ctx context.Context) error {
	c.mu.Lock()

	if c.state == containerSetup || c.state == containerStopped {
		c.mu.Unlock()
		return nil
	}

	c.state = containerStopping
	stopFns := slices.Clone(c.stopFns)
	stoppingFns := slices.Clone(c.stoppingFns)
	c.mu.Unlock()

	var firstErr error

	// Stopping hooks run before waiting, so they can unblock managed tasks.
	for _, hook := range slices.Backward(stoppingFns) {
		c.logStep(ctx, slog.LevelDebug, "container step starting", "stopping", hook.name)

		if stoppingErr := hook.fn(ctx, c); stoppingErr != nil {
			c.logStep(
				ctx,
				slog.LevelError,
				"container step error",
				"stopping",
				hook.name,
				slog.String("err", stoppingErr.Error()),
			)

			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %s: %w", c.name, hook.name, stoppingErr)
			}
			continue
		}

		c.logStep(ctx, slog.LevelDebug, "container step completed", "stopping", hook.name)
	}

	c.wg.Wait()

	// Stop hooks run after managed tasks exit, so state is safe to finalize.
	for _, hook := range slices.Backward(stopFns) {
		c.logStep(ctx, slog.LevelDebug, "container step starting", "stop", hook.name)

		if stopErr := hook.fn(ctx, c); stopErr != nil {
			c.logStep(
				ctx,
				slog.LevelError,
				"container step error",
				"stop",
				hook.name,
				slog.String("err", stopErr.Error()),
			)

			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %s: %w", c.name, hook.name, stopErr)
			}
			continue
		}

		c.logStep(ctx, slog.LevelDebug, "container step completed", "stop", hook.name)
	}

	c.mu.Lock()
	c.state = containerStopped
	c.mu.Unlock()

	c.runtime.log(ctx, slog.LevelInfo, "container stopped", slog.String("container", c.name))
	return firstErr
}

// start is called only by Runtime.Run, in attachment order.
func (c *Container) start() error {
	c.mu.Lock()
	if c.state != containerInitialized {
		c.mu.Unlock()
		panic("gx: container started more than once: " + c.name)
	}

	c.state = containerStarting
	startFns := slices.Clone(c.startFns)
	c.mu.Unlock()

	c.runtime.log(
		c.runtime.ctx,
		slog.LevelInfo,
		"container starting",
		slog.String("container", c.name),
	)

	// Hooks may Set dependencies and start immediate tasks with Run.
	for _, hook := range startFns {
		// A task may have failed while this hook was running.
		if c.runtime.ctx.Err() != nil {
			return nil
		}

		c.logStep(c.runtime.ctx, slog.LevelDebug, "container step starting", "start", hook.name)

		if err := hook.fn(c); err != nil {
			c.logStep(
				c.runtime.ctx,
				slog.LevelError,
				"container step error",
				"start",
				hook.name,
				slog.String("err", err.Error()),
			)
			return fmt.Errorf("%s: %s: %w", c.name, hook.name, err)
		}

		c.logStep(c.runtime.ctx, slog.LevelDebug, "container step completed", "start", hook.name)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.runtime.ctx.Err() != nil {
		return nil
	}

	// Start tasks registered during construction or OnInit after OnStart succeeds.
	for _, task := range c.tasks {
		c.startTaskLocked(task)
	}

	c.tasks = nil
	c.state = containerRunning
	return nil
}

// runningTasks returns a snapshot of managed work that has not exited.
func (c *Container) runningTasks() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return slices.Clone(c.running)
}

// startTaskLocked requires c.mu and a live Runtime runner.
func (c *Container) startTaskLocked(task containerTask) {
	c.running = append(c.running, task.name)
	c.wg.Add(1)
	c.runtime.runner.Run(func() error {
		defer c.wg.Done()
		defer c.stopTask(task.name)

		c.runtime.log(
			c.runtime.ctx,
			slog.LevelDebug,
			"container task starting",
			slog.String("container", c.name),
			slog.String("task", task.name),
		)

		err := task.fn(c.runtime.ctx)
		// canceled means Runtime canceled the task's context.
		canceled := c.runtime.ctx.Err() != nil && errors.Is(err, context.Canceled)
		if err != nil && !canceled {
			c.runtime.log(
				c.runtime.ctx,
				slog.LevelError,
				"container task error",
				slog.String("container", c.name),
				slog.String("task", task.name),
				slog.String("err", err.Error()),
			)
			return fmt.Errorf("%s: %s: %w", c.name, task.name, err)
		}

		c.runtime.log(
			c.runtime.ctx,
			slog.LevelDebug,
			"container task stopped",
			slog.String("container", c.name),
			slog.String("task", task.name),
		)
		return nil
	})
}

// stopTask removes managed work after it exits.
func (c *Container) stopTask(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, task := range c.running {
		if task == name {
			c.running = slices.Delete(c.running, i, i+1)
			return
		}
	}
}

// store is a shared, type-keyed storage for runtime data.
type store struct {
	m  map[reflect.Type]any
	mu sync.RWMutex
}

// newStore creates a type-keyed store.
func newStore() *store {
	return &store{
		m: make(map[reflect.Type]any),
	}
}

// get retrieves a value by type from the store.
// It returns the value and whether it was found.
func (s *store) get(t reflect.Type) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	v, ok := s.m[t]
	return v, ok
}

// set stores a value by type in the store.
// It returns false if a value of the same type already exists.
func (s *store) set(t reflect.Type, v any) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.m == nil {
		s.m = make(map[reflect.Type]any)
	}

	if _, exists := s.m[t]; exists {
		return false
	}

	s.m[t] = v
	return true
}

// getValue retrieves a value of type T from value.
// It panics if the value is not of type T.
func getValue[T any](value any) T {
	if value == nil {
		var zero T
		return zero
	}

	v, ok := value.(T)
	if !ok {
		panic(fmt.Sprintf(
			"gx: internal dependency type mismatch for %s: got %T",
			reflect.TypeFor[T](),
			value,
		))
	}

	return v
}
