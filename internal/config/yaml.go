package config

import (
	"bytes"
	"errors"
	"io"

	"go.yaml.in/yaml/v3"
)

func decodeYAML(data []byte, destination any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("configuration must contain exactly one YAML document")
		}
		return err
	}
	return nil
}
