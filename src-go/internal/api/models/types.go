package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSONMap is a helper type for JSONB columns that are arbitrary key-value maps.
type JSONMap map[string]any

func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	b, err := json.Marshal(j)
	return string(b), err
}

func (j *JSONMap) Scan(src any) error {
	if src == nil {
		*j = nil
		return nil
	}
	var bytes []byte
	switch v := src.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("JSONMap: unsupported type %T", src)
	}
	return json.Unmarshal(bytes, j)
}

// StringArray is a helper for PostgreSQL text[] columns stored as JSON arrays.
type StringArray []string

func (s StringArray) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	b, err := json.Marshal(s)
	return string(b), err
}

func (s *StringArray) Scan(src any) error {
	if src == nil {
		*s = nil
		return nil
	}
	var bytes []byte
	switch v := src.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("StringArray: unsupported type %T", src)
	}
	return json.Unmarshal(bytes, s)
}
