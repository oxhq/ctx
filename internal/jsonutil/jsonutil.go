package jsonutil

import "encoding/json"

func MarshalStable(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
