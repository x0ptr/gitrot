package state

import (
	"encoding/json"
	"fmt"
	"os"
)

type File struct {
	Acknowledged map[string]string `json:"acknowledged"`
}

func Load(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return File{Acknowledged: map[string]string{}}, nil
		}
		return File{}, fmt.Errorf("read state file: %w", err)
	}

	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return File{}, fmt.Errorf("parse state file: %w", err)
	}
	if f.Acknowledged == nil {
		f.Acknowledged = map[string]string{}
	}
	return f, nil
}

func Save(path string, f File) error {
	if f.Acknowledged == nil {
		f.Acknowledged = map[string]string{}
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state file: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write state file: %w", err)
	}
	return nil
}
