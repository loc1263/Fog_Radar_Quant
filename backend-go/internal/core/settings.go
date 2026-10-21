// Package core reúne lo transversal del servicio: configuración, errores, auth,
// límites y registro de jobs.
package core

import (
	"bufio"
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

// Settings es la configuración del proceso.
type Settings struct {
	Env      string // dev | test | prod (en prod no se sirven /docs ni /openapi)
	AppName  string
	LogLevel string
	DataDir  string
	Timezone string
	Addr     string // dirección de escucha de `fogradar api`

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
	RetencionDias  int     // días de datos que se conservan en data/ (0 = no borrar nada)
	PipelinePausaS float64 // pausa entre un pipeline y el siguiente de una secuencia

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
		Env:      strings.ToLower(get("ENV", "dev")),
		AppName:  get("APP_NAME", "Fog Radar Quant API"),
		LogLevel: strings.ToUpper(get("LOG_LEVEL", "INFO")),
		Timezone: get("TIMEZONE", "America/Santiago"),
		DataDir:  get("DATA_DIR", filepath.Join("..", "data")),
		Addr:     get("ADDR", ":8000"),
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
	s.RetencionDias = int(num("RETENCION_DIAS", 7, 0, 3650))
	if perr != nil {
		return nil, perr
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
	return nil
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
