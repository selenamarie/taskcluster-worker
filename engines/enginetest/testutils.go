package enginetest

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	rt "runtime"
	"strings"
	"sync"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
)

func fmtPanic(a ...interface{}) {
	panic(fmt.Sprintln(a...))
}

func nilOrPanic(err error, a ...interface{}) {
	if err != nil {
		fmtPanic(append(a, err)...)
	}
}

func evalNilOrPanic(f func() error, a ...interface{}) {
	nilOrPanic(f(), a...)
}

func assert(condition bool, a ...interface{}) {
	if !condition {
		fmtPanic(a...)
	}
}

// Type can embed so that we can reuse ensure engine
type engineProvider struct {
	sync.Mutex
	engine      engines.Engine
	environment *runtime.Environment
}

func (p *engineProvider) ensureEngine(engineName string) {
	p.Lock()
	defer p.Unlock()
	if p.engine != nil {
		return
	}
	// Create a runtime environment
	p.environment = newTestEnvironment()
	// Find EngineProvider
	engineProvider := extpoints.EngineProviders.Lookup(engineName)
	if engineProvider == nil {
		fmtPanic("Couldn't find EngineProvider: ", engineName)
	}
	// Create Engine instance
	engine, err := engineProvider.NewEngine(extpoints.EngineOptions{
		Environment: p.environment,
		Log:         p.environment.Log.WithField("engine", engineName),
	})
	nilOrPanic(err, "Failed to create Engine")
	p.engine = engine
}

func (p *engineProvider) newTestTaskContext() (*runtime.TaskContext, *runtime.TaskContextController) {
	ctx, control, err := runtime.NewTaskContext(p.environment.TemporaryStorage.NewFilePath())
	nilOrPanic(err, "Failed to create new TaskContext")
	return ctx, control
}

// NewTestEnvironment creates a new Environment suitable for use in tests.
//
// This function should only be used in testing
func newTestEnvironment() *runtime.Environment {
	storage, err := runtime.NewTemporaryStorage(os.TempDir())
	nilOrPanic(err, "Failed to create temporary storage at: ", os.TempDir())

	folder, err := storage.NewFolder()
	nilOrPanic(err, "Failed to create temporary storage folder")

	// Set finalizer so that we always get the temporary folder removed.
	// This is should really only be used in tests, otherwise it would better to
	// call Remove() manually.
	rt.SetFinalizer(folder, func(f runtime.TemporaryFolder) {
		f.Remove()
	})

	logger, err := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating logger. %s", err)
		os.Exit(1)
	}

	return &runtime.Environment{
		GarbageCollector: &gc.GarbageCollector{},
		TemporaryStorage: folder,
		Log:              logger,
	}
}

func parseTestPayload(engine engines.Engine, payload string) interface{} {
	jsonPayload := map[string]json.RawMessage{}
	err := json.Unmarshal([]byte(payload), &jsonPayload)
	nilOrPanic(err, "Payload parsing failed: ", payload)
	p, err := engine.PayloadSchema().Parse(jsonPayload)
	nilOrPanic(err, "Payload validation failed: ", payload)
	return p
}

func buildRunSandbox(b engines.SandboxBuilder) bool {
	// Start sandbox and wait for result
	sandbox, err := b.StartSandbox()
	nilOrPanic(err, "Failed to start sandbox")

	// Wait for result
	resultSet, err := sandbox.WaitForResult()
	nilOrPanic(err, "WaitForResult failed")

	// Get result and dispose ResultSet
	defer evalNilOrPanic(resultSet.Dispose, "Failed to dispose of ResultSet")
	return resultSet.Success()
}

// run helper
type run struct {
	provider       *engineProvider
	context        *runtime.TaskContext
	control        *runtime.TaskContextController
	sandboxBuilder engines.SandboxBuilder
	sandbox        engines.Sandbox
	resultSet      engines.ResultSet
	logReader      io.ReadCloser
	closedLog      bool
}

func (p *engineProvider) NewRun(engine string) *run {
	p.ensureEngine(engine)
	r := run{}
	r.provider = p
	r.context, r.control = p.newTestTaskContext()
	return &r
}

func (r *run) NewSandboxBuilder(payload string) {
	assert(r.sandboxBuilder == nil, "NewSandboxBuilder already called!")
	sandboxBuilder, err := r.provider.engine.NewSandboxBuilder(engines.SandboxOptions{
		TaskContext: r.context,
		Payload:     parseTestPayload(r.provider.engine, payload),
	})
	r.sandboxBuilder = sandboxBuilder
	nilOrPanic(err, "Error creating SandboxBuilder")
}

func (r *run) StartSandbox() {
	assert(r.sandbox == nil, "StartSandbox already called!")
	sandbox, err := r.sandboxBuilder.StartSandbox()
	nilOrPanic(err, "Failed to start sandbox")
	r.sandbox = sandbox
}

func (r *run) WaitForResult() bool {
	assert(r.resultSet == nil, "WaitForResult already called!")
	resultSet, err := r.sandbox.WaitForResult()
	nilOrPanic(err, "WaitForResult failed")
	r.resultSet = resultSet
	r.closedLog = true
	nilOrPanic(r.control.CloseLog(), "Failed to close log writer")
	return r.resultSet.Success()
}

func (r *run) OpenLogReader() {
	assert(r.logReader == nil, "OpenLogReader already called!")
	logReader, err := r.context.NewLogReader()
	nilOrPanic(err, "Failed to open log reader")
	r.logReader = logReader
}

func (r *run) ReadLog() string {
	reader, err := r.context.NewLogReader()
	defer reader.Close()
	nilOrPanic(err, "Failed to open log reader")
	data, err := ioutil.ReadAll(reader)
	nilOrPanic(err, "Failed to read log")
	return string(data)
}

func (r *run) Dispose() {
	if r.sandboxBuilder != nil {
		nilOrPanic(r.sandboxBuilder.Discard(), "")
		r.sandboxBuilder = nil
	}
	if r.sandbox != nil {
		nilOrPanic(r.sandbox.Abort(), "")
		r.sandbox = nil
	}
	if r.resultSet != nil {
		nilOrPanic(r.resultSet.Dispose(), "")
		r.resultSet = nil
	}
	if r.logReader != nil {
		nilOrPanic(r.logReader.Close(), "")
		r.logReader = nil
	}
	if !r.closedLog {
		r.closedLog = true
		nilOrPanic(r.control.CloseLog(), "Failed to close log writer")
	}
	nilOrPanic(r.control.Dispose(), "")
}

// Auxiliary composite methods

func (r *run) buildRunSandbox() bool {
	r.StartSandbox()
	return r.WaitForResult()
}

func (r *run) GrepLog(needle string) bool {
	return strings.Contains(r.ReadLog(), needle)
}
