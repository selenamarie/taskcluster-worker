package enginetest

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"sync"

	"github.com/taskcluster/taskcluster-worker/engines"
)

// The ArtifactTestCase contains information sufficient to test artifact
// extration from an engine.
type ArtifactTestCase struct {
	engineProvider
	Engine string
	// Text to search for in files
	Text string
	// Path of a file containing the Text string above
	TextFilePath string
	// Path to a file that doesn't exist, and will return ErrResourceNotFound
	FileNotFoundPath string
	// Path to a folder that doesn't exist, and will return ErrResourceNotFound
	FolderNotFoundPath string
	// Files to expect in NestedFolderPath
	NestedFolderFiles []string
	// Path to a folder that contains files NestedFolderFiles each containing
	// Text
	NestedFolderPath string
	// Payload that will generate a ResultSet containing paths described above.
	Payload string
}

// TestExtractTextFile checks that TextFilePath contains Text
func (c *ArtifactTestCase) TestExtractTextFile() {
	r := c.NewRun(c.Engine)
	defer r.Dispose()
	r.NewSandboxBuilder(c.Payload)
	assert(r.buildRunSandbox(), "Task failed to run, payload: ", c.Payload)

	reader, err := r.resultSet.ExtractFile(c.TextFilePath)
	nilOrPanic(err, "Failed to ExtractFile: ", c.TextFilePath)
	data, err := ioutil.ReadAll(reader)
	nilOrPanic(err, "Failed to read file: ", c.TextFilePath)
	assert(strings.Contains(string(data), c.Text),
		"Expected ", c.TextFilePath, " to contain '", c.Text, "', got ",
		string(data))
}

// TestExtractFileNotFound checks that FileNotFoundPath returns
// ErrResourceNotFound
func (c *ArtifactTestCase) TestExtractFileNotFound() {
	r := c.NewRun(c.Engine)
	defer r.Dispose()
	r.NewSandboxBuilder(c.Payload)
	assert(r.buildRunSandbox(), "Task failed to run, payload: ", c.Payload)

	_, err := r.resultSet.ExtractFile(c.FileNotFoundPath)
	assert(err == engines.ErrResourceNotFound, "Expected ErrResourceNotFound ",
		"but got :", err)
}

// TestExtractFolderNotFound checks that FolderNotFoundPath returns
// ErrResourceNotFound
func (c *ArtifactTestCase) TestExtractFolderNotFound() {
	r := c.NewRun(c.Engine)
	defer r.Dispose()
	r.NewSandboxBuilder(c.Payload)
	assert(r.buildRunSandbox(), "Task failed to run, payload: ", c.Payload)

	err := r.resultSet.ExtractFolder(c.FolderNotFoundPath, func(
		path string, reader io.ReadCloser,
	) error {
		return errors.New("File was found, didn't expect that!!!")
	})
	assert(err == engines.ErrResourceNotFound, "Expected ErrResourceNotFound ",
		"but got :", err)
}

// TestExtractNestedFolderPath checks FolderNotFoundPath contains files
// NestedFolderFiles
func (c *ArtifactTestCase) TestExtractNestedFolderPath() {
	r := c.NewRun(c.Engine)
	defer r.Dispose()
	r.NewSandboxBuilder(c.Payload)
	assert(r.buildRunSandbox(), "Task failed to run, payload: ", c.Payload)

	m := sync.Mutex{}
	files := []string{}
	err := r.resultSet.ExtractFolder(c.NestedFolderPath, func(
		path string, reader io.ReadCloser,
	) error {
		m.Lock()
		files = append(files, path)
		m.Unlock()
		data, err := ioutil.ReadAll(reader)
		if err != nil {
			return fmt.Errorf("Error reading %s error: %s", path, err)
		}
		if !strings.Contains(string(data), c.Text) {
			return fmt.Errorf("Reading %s but didn't find: %s", path, string(data))
		}
		return nil
	})
	nilOrPanic(err, "Error handling files from folder")

	// Check that NestedFolderFiles was found
	for _, f := range c.NestedFolderFiles {
		found := false
		for _, f2 := range files {
			if f == f2 {
				found = true
			}
		}
		assert(found, "Didn't get file: ", f)
	}

	// Check that only NestedFolderFiles was found
	for _, f := range files {
		found := false
		for _, f2 := range c.NestedFolderFiles {
			if f == f2 {
				found = true
			}
		}
		assert(found, "Found file ", f, " but it wasn't declared in: ", c.NestedFolderFiles)
	}
}

// TestExtractFolderHandlerInterrupt checks that errors in handler given to
// ExtractFolder causes ErrHandlerInterrupt
func (c *ArtifactTestCase) TestExtractFolderHandlerInterrupt() {
	r := c.NewRun(c.Engine)
	defer r.Dispose()
	r.NewSandboxBuilder(c.Payload)
	assert(r.buildRunSandbox(), "Task failed to run, payload: ", c.Payload)

	err := r.resultSet.ExtractFolder(c.NestedFolderPath, func(
		path string, reader io.ReadCloser,
	) error {
		return errors.New("Error that should interrupt ExtractFolder")
	})
	assert(err == engines.ErrHandlerInterrupt,
		"Expected ErrHandlerInterrupt from ExtractFolder, got", err)
}

// Test runs all test cases in parallel
func (c *ArtifactTestCase) Test() {
	c.ensureEngine(c.Engine)
	wg := sync.WaitGroup{}
	wg.Add(5)
	go func() { c.TestExtractTextFile(); wg.Done() }()
	go func() { c.TestExtractFileNotFound(); wg.Done() }()
	go func() { c.TestExtractFolderNotFound(); wg.Done() }()
	go func() { c.TestExtractNestedFolderPath(); wg.Done() }()
	go func() { c.TestExtractFolderHandlerInterrupt(); wg.Done() }()
	wg.Wait()
}
