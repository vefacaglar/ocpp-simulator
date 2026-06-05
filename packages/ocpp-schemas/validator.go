package ocppschemas

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

//go:embed v16/*.json v201/*.json
var schemaFS embed.FS

type Version string

// Canonical Version constants. These match the protocol.Version1_6 /
// protocol.Version2_0_1 strings used as Factory keys and MQTT topic
// segments. The previous value for V16 was "1.6" (no J) which was
// inconsistent with the protocol package and the web frontend. A
// single source of truth removes a class of stringly-typed bugs at
// the validator boundary.
const (
	V16  Version = "1.6J"
	V201 Version = "2.0.1"
)

type Direction string

const (
	Request  Direction = "Request"
	Response Direction = "Response"
)

var schemaCache = map[string]*jsonschema.Schema{}

func loadSchema(version Version, action string, dir Direction) (*jsonschema.Schema, error) {
	key := fmt.Sprintf("%s/%s/%s", version, action, dir)
	if s, ok := schemaCache[key]; ok {
		return s, nil
	}

	filename := action
	if dir == Response {
		filename += "Response"
	}

	var path string
	switch version {
	case V16:
		path = fmt.Sprintf("v16/%s.json", filename)
	case V201:
		path = fmt.Sprintf("v201/%s.json", filename)
	default:
		return nil, fmt.Errorf("ocpp-schemas: unknown version %s", version)
	}

	data, err := schemaFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ocpp-schemas: schema file not found: %s", path)
	}

	schema, err := jsonschema.CompileString(path, string(data))
	if err != nil {
		return nil, fmt.Errorf("ocpp-schemas: compile error for %s: %w", path, err)
	}

	schemaCache[key] = schema
	return schema, nil
}

// Validate checks a payload against the official OCPP schema for the given
// version, action, and direction (request vs. response).
func Validate(version Version, action string, dir Direction, payload []byte) error {
	schema, err := loadSchema(version, action, dir)
	if err != nil {
		return err
	}

	var v interface{}
	if err := json.Unmarshal(payload, &v); err != nil {
		return fmt.Errorf("ocpp-schemas: invalid JSON: %w", err)
	}

	if err := schema.Validate(v); err != nil {
		return fmt.Errorf("ocpp-schemas: validation failed: %w", err)
	}

	return nil
}

// SchemaAction returns the action name from a schema filename.
func SchemaAction(filename string) string {
	name := strings.TrimSuffix(filename, ".json")
	name = strings.TrimSuffix(name, "Response")
	return name
}
