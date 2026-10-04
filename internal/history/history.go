package history

import (
	"encoding/json"
	"errors"
	"globalspeed/internal/speed"
	"os"
	"path/filepath"
	"sync"
)

var mu sync.Mutex

// Path returns the shared desktop/CLI history file location.
func Path() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "globalspeed", "history.json"), nil
}
func Load() ([]speed.Result, error) { mu.Lock(); defer mu.Unlock(); return load() }
func load() ([]speed.Result, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return []speed.Result{}, nil
	}
	if err != nil {
		return nil, err
	}
	var results []speed.Result
	err = json.Unmarshal(b, &results)
	return results, err
}
func Save(result speed.Result) error {
	mu.Lock()
	defer mu.Unlock()
	all, err := load()
	if err != nil {
		return err
	}
	all = append([]speed.Result{result}, all...)
	if len(all) > 100 {
		all = all[:100]
	}
	p, err := Path()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(p), "history-*.json")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
