package api

import "os"

// hasData: la carpeta del módulo existe y tiene algo que no empiece con ".".
func hasData(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if len(e.Name()) > 0 && e.Name()[0] != '.' {
			return true
		}
	}
	return false
}
