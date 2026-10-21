// Package core reúne lo transversal del servicio: configuración, errores, auth,
// límites, registro de jobs y acceso a history.db.
package core

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // zonas horarias embebidas: Windows no trae base IANA
)

// Version de la API. Se sobrescribe con -ldflags "-X fogradar/internal/core.Version=…".
var Version = "0.1.0"

// clavesDeEjemplo son valores de .env.example: nunca válidos en prod.
var clavesDeEjemplo = map[string]bool{"cambiar-por-una-clave-larga-aleatoria": true}

// Settings es la configuración del proceso.
type Settings struct {
	Env       string // dev | test | prod
	AppName   string
	LogLevel  string
	APIKey    string // vacío = sin autenticación (solo dev/test)
	Origins   []string
	RateLimit string // p. ej. "300/minute"
	// RateLimitInTests activa el rate limit con Env=test.
	RateLimitInTests bool
	DataDir          string
	Timezone         string
	Addr             string // dirección de escucha (solo Go)

	// Yahoo Finance (internal/yahoo): reglas globales del proceso.
	YahooMinIntervaloS float64 // intervalo mínimo entre inicios de request
	YahooJitterS       float64 // + espera aleatoria de 0 a este valor
	Yahoo429Reintentos int     // reintentos ante HTTP 429 (backoff exponencial)
	YahooBackoffBaseS  float64
	YahooBackoffMaxS   float64
	YahooRedReintentos int // reintentos de red transitorios
	// Espaciado adaptivo: parte en YahooIntervaloRapidoS y, ante un 429, se ensancha (nunca menos que
	// YahooMinIntervaloS) y vuelve a bajar solo.
	YahooAdaptativo       bool
	YahooIntervaloRapidoS float64
	// Corridas batch (internal/jobs).
	PipelinePausaS          float64 // pausa entre un pipeline y el siguiente de una secuencia
	PipelineReintentos      int     // reintentos de un pipeline que falla en una corrida programada
	PipelineEsperaReintento float64 // espera entre intentos
	PipelineLockEsperaS     float64 // cuánto espera una corrida programada si otra secuencia tiene el lock
	AlertWebhookURL         string  // Slack/Discord/Teams: aviso cuando un pipeline falla
	FredAPIKey              string

	// YahooConcurrencia es cuántos requests pueden estar en vuelo a la vez (inicios siempre espaciados).
	YahooConcurrencia int

	Loc *time.Location
}

// ModuleKey es la clave de un módulo (carpeta bajo DataDir).
type ModuleKey string

// IsProd indica si corre en producción.
func (s *Settings) IsProd() bool { return s.Env == "prod" }

// ModuleDataDir es data/<modulo>.
func (s *Settings) ModuleDataDir(m ModuleKey) string { return filepath.Join(s.DataDir, string(m)) }

// JobsDir es data/_jobs.
func (s *Settings) JobsDir() string { return filepath.Join(s.DataDir, "_jobs") }

// LoadSettings lee variables de entorno y, como respaldo, el archivo .env
// (ENV_FILE, o ./.env, o backend-go/.env). Una variable de entorno gana al .env.
func LoadSettings() (*Settings, error) {
	return loadSettings(os.LookupEnv, envFileCandidates())
}

func envFileCandidates() []string {
	if p := os.Getenv("ENV_FILE"); p != "" {
		return []string{p}
	}
	return []string{".env", filepath.Join("backend-go", ".env")}
}

func loadSettings(lookup func(string) (string, bool), files []string) (*Settings, error) {
	dotenv := map[string]string{}
	for _, f := range files {
		if m, err := parseDotenv(f); err == nil {
			dotenv = m
			break
		}
	}
	get := func(key, def string) string {
		if v, ok := lookup(key); ok {
			return v
		}
		if v, ok := dotenv[key]; ok {
			return v
		}
		return def
	}

	s := &Settings{
		Env:       strings.ToLower(get("ENV", "dev")),
		AppName:   get("APP_NAME", "Fog Radar Quant API"),
		LogLevel:  strings.ToUpper(get("LOG_LEVEL", "INFO")),
		APIKey:    get("API_KEY", ""),
		RateLimit: get("RATE_LIMIT_DEFAULT", "300/minute"),
		Timezone:  get("TIMEZONE", "America/Santiago"),
		DataDir:   get("DATA_DIR", filepath.Join("..", "data")),
		Addr:      get("ADDR", ":8000"),
	}
	var perr error
	num := func(key string, def float64, min, max float64) float64 {
		v := get(key, "")
		if v == "" {
			return def
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f < min || f > max {
			perr = fmt.Errorf("%s inválido: %q (rango %g..%g)", key, v, min, max)
			return def
		}
		return f
	}
	s.YahooMinIntervaloS = num("YAHOO_MIN_INTERVALO_S", 0.35, 0, 3600)
	s.YahooJitterS = num("YAHOO_JITTER_S", 0.25, 0, 3600)
	s.Yahoo429Reintentos = int(num("YAHOO_REINTENTOS_429", 3, 0, 10))
	s.YahooBackoffBaseS = num("YAHOO_BACKOFF_BASE_S", 5, 0, 3600)
	s.YahooBackoffMaxS = num("YAHOO_BACKOFF_MAX_S", 60, 0, 3600)
	s.YahooRedReintentos = int(num("YAHOO_REINTENTOS_RED", 2, 0, 10))
	s.YahooConcurrencia = int(num("YAHOO_CONCURRENCIA", 4, 1, 32))
	s.YahooAdaptativo = true
	if v := get("YAHOO_ADAPTATIVO", ""); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("YAHOO_ADAPTATIVO inválido: %q", v)
		}
		s.YahooAdaptativo = b
	}
	s.YahooIntervaloRapidoS = num("YAHOO_INTERVALO_RAPIDO_S", 0.2, 0.01, 3600)
	s.PipelinePausaS = num("PIPELINE_PAUSA_S", 60, 0, 86400)
	s.PipelineReintentos = int(num("PIPELINE_REINTENTOS", 1, 0, 20))
	s.PipelineEsperaReintento = num("PIPELINE_ESPERA_REINTENTO_S", 600, 0, 86400)
	s.PipelineLockEsperaS = num("PIPELINE_LOCK_ESPERA_S", 6*3600, 0, 7*86400)
	s.AlertWebhookURL = get("ALERT_WEBHOOK_URL", "")
	s.FredAPIKey = get("FRED_API_KEY", "")
	if perr != nil {
		return nil, perr
	}
	s.Origins = splitCSV(get("ALLOWED_ORIGINS", "http://localhost:5173"))
	if v := get("RATE_LIMIT_IN_TESTS", ""); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("RATE_LIMIT_IN_TESTS inválido: %q", v)
		}
		s.RateLimitInTests = b
	}
	return s, s.finish()
}

func (s *Settings) finish() error {
	switch s.Env {
	case "dev", "test", "prod":
	default:
		return fmt.Errorf("ENV inválido: %q (dev | test | prod)", s.Env)
	}
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return fmt.Errorf("TIMEZONE inválida %q: %w", s.Timezone, err)
	}
	s.Loc = loc
	DefaultLoc = loc
	if abs, err := filepath.Abs(s.DataDir); err == nil {
		s.DataDir = abs
	}
	if _, _, err := ParseLimit(s.RateLimit); err != nil {
		return fmt.Errorf("RATE_LIMIT_DEFAULT: %w", err)
	}
	if s.Env == "prod" {
		if len(s.APIKey) < 32 || clavesDeEjemplo[s.APIKey] || distinctRunes(s.APIKey) < 12 {
			return errors.New("en prod API_KEY es obligatoria, de al menos 32 caracteres y aleatoria")
		}
		for _, o := range s.Origins {
			if strings.Contains(o, "localhost") || strings.Contains(o, "127.0.0.1") {
				return errors.New("en prod ALLOWED_ORIGINS no puede incluir localhost")
			}
		}
	}
	return nil
}

func distinctRunes(s string) int {
	seen := map[rune]bool{}
	for _, r := range s {
		seen[r] = true
	}
	return len(seen)
}

func splitCSV(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseDotenv lee un archivo KEY=VALUE (comentarios con #, comillas opcionales).
// Las claves se normalizan a mayúsculas, para que la búsqueda no distinga mayúsculas.
func parseDotenv(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), bom))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
			v = v[1 : len(v)-1]
		} else if i := strings.Index(v, " #"); i >= 0 {
			v = strings.TrimSpace(v[:i])
		}
		out[strings.ToUpper(strings.TrimSpace(k))] = v
	}
	return out, sc.Err()
}

// bom es la marca de orden de bytes UTF-8 que a veces antepone el Bloc de notas.
var bom = string(rune(0xFEFF))
