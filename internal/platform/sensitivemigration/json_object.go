package sensitivemigration

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
)

var ErrInvalidJSONObject = errors.New("sensitive migration invalid JSON object")

func DecodeUniqueObject(raw string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return nil, ErrInvalidJSONObject
	}
	object := map[string]json.RawMessage{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, ErrInvalidJSONObject
		}
		key, ok := token.(string)
		if !ok {
			return nil, ErrInvalidJSONObject
		}
		if _, duplicate := object[key]; duplicate {
			return nil, ErrInvalidJSONObject
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, ErrInvalidJSONObject
		}
		object[key] = value
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return nil, ErrInvalidJSONObject
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, ErrInvalidJSONObject
	}
	return object, nil
}
