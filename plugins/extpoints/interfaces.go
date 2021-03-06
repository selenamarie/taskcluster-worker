package extpoints

import (
	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// PluginOptions is a wrapper for the arguments/options given when instantiating
// a Plugin using PluginProvider.
//
// We wrap all arguments so that we can add additional properties without
// breaking source compatibility with older plugins.
type PluginOptions struct {
	environment *runtime.Environment
	engine      *engines.Engine
	log         *logrus.Entry
}

// The PluginProvider interface must be implemented and registered by anyone
// implementing a Plugin.
//
// If an implementor can determine that a plugin isn't available at compile-time
// it is preferred not to register the plugin.
type PluginProvider interface {
	NewPlugin(options PluginOptions) (plugins.Plugin, error)

	// ConfigSchema returns the CompositeSchema that represents the plugin
	// config.
	ConfigSchema() runtime.CompositeSchema
}
