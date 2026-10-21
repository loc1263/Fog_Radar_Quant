package core

import "crypto/subtle"

// APIKeyHeader es el header de autenticación de la API (`API_KEY`).
const APIKeyHeader = "X-API-Key"

// KeyValid compara en tiempo constante. Sin clave configurada (solo dev/test) acepta todo.
func (s *Settings) KeyValid(provided string) bool {
	if s.APIKey == "" {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(s.APIKey)) == 1
}
