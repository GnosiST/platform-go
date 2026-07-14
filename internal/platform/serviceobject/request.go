package serviceobject

import (
	"encoding/json"
	"fmt"
	"io"
)

func DecodeQueryRequest(reader io.Reader) (QueryRequest, error) {
	var request QueryRequest
	if err := decodeStrictJSON(reader, &request); err != nil {
		return QueryRequest{}, fmt.Errorf("%w", ErrRequestInvalid)
	}
	return request, nil
}

func DecodeCommandRequest(reader io.Reader) (CommandRequest, error) {
	var request CommandRequest
	if err := decodeStrictJSON(reader, &request); err != nil {
		return CommandRequest{}, fmt.Errorf("%w", ErrRequestInvalid)
	}
	return request, nil
}

func decodeStrictJSON(reader io.Reader, target any) error {
	decoder := json.NewDecoder(io.LimitReader(reader, 1<<20))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}
