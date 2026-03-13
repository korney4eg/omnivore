// Package scalar implements custom GraphQL scalars that match the TypeScript API.
package scalar

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Date is a GraphQL scalar backed by time.Time.
// It marshals as an RFC3339 string to match the TypeScript API.
type Date time.Time

func (d Date) MarshalGQL(w io.Writer) {
	t := time.Time(d)
	_, _ = fmt.Fprintf(w, `"%s"`, t.UTC().Format(time.RFC3339))
}

func (d *Date) UnmarshalGQL(v any) error {
	switch s := v.(type) {
	case string:
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			// Try without timezone
			t, err = time.Parse("2006-01-02T15:04:05", s)
			if err != nil {
				return fmt.Errorf("date scalar: %w", err)
			}
		}
		*d = Date(t)
		return nil
	case int64:
		*d = Date(time.UnixMilli(s))
		return nil
	default:
		return fmt.Errorf("date scalar: unexpected type %T", v)
	}
}

// JSON is a GraphQL scalar that passes arbitrary JSON values through as-is.
type JSON json.RawMessage

func (j JSON) MarshalGQL(w io.Writer) {
	if j == nil {
		_, _ = io.WriteString(w, "null")
		return
	}
	_, _ = w.Write(j)
}

func (j *JSON) UnmarshalGQL(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("json scalar: %w", err)
	}
	*j = JSON(b)
	return nil
}
