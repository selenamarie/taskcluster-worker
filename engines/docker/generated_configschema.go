// This source code file is AUTO-GENERATED by github.com/taskcluster/jsonschema2go

package docker

import "github.com/taskcluster/taskcluster-worker/runtime"

type (
	// Config applicable to docker engine
	config struct {

		// Root Volume blah blah
		RootVolume string `json:"rootVolume"`
	}
)

var configSchema = func() runtime.CompositeSchema {
	schema, err := runtime.NewCompositeSchema(
		"config",
		`
		{
		  "$schema": "http://json-schema.org/draft-04/schema#",
		  "additionalProperties": false,
		  "description": "Config applicable to docker engine",
		  "properties": {
		    "rootVolume": {
		      "description": "Root Volume blah blah",
		      "title": "Root Volume",
		      "type": "string"
		    }
		  },
		  "required": [
		    "rootVolume"
		  ],
		  "title": "Config",
		  "type": "object"
		}
		`,
		true,
		func() interface{} {
			return &config{}
		},
	)
	if err != nil {
		panic(err)
	}
	return schema
}()
