package plugins

import (
	"encoding/json"
	"fmt"
)

func approvedFamilyPlugin(id string) bool {
	return id == "nd-lyrics" || id == "apple-music" || id == "coverartarchive"
}
func validateFamilyConfig(id, raw string) error {
	if id != "nd-lyrics" {
		return nil
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return err
	}
	if enabled, _ := v["writeLyrics"].(bool); enabled {
		return fmt.Errorf("family lyrics sidecar writes are prohibited")
	}
	return nil
}
