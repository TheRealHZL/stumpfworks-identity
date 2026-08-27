package clientupdate

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

const DefaultStatePath = "/var/lib/stumpfworks-badge/update-state.json"

type UpdateState struct {
	Version           string    `json:"version"`
	Status            string    `json:"status"`
	UpdatedAt         time.Time `json:"updated_at"`
	RollbackAvailable bool      `json:"rollback_available"`
}

func WriteUpdateState(path string, state UpdateState) error {
	if path == "" || !validVersion(state.Version) || (state.Status != "success" && state.Status != "failed") || state.UpdatedAt.IsZero() {
		return errors.New("invalid update state")
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err = writeExclusive(temporary, raw, 0644); err != nil {
		return err
	}
	if err = os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func ReadUpdateState(path string) (*UpdateState, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(raw) > 4096 {
		return nil, errors.New("update state exceeds size limit")
	}
	var state UpdateState
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&state); err != nil {
		return nil, err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return nil, errors.New("update state contains trailing data")
	}
	if !validVersion(state.Version) || (state.Status != "success" && state.Status != "failed") || state.UpdatedAt.IsZero() {
		return nil, errors.New("invalid update state")
	}
	return &state, nil
}
