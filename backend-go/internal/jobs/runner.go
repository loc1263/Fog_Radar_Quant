package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"fogradar/internal/core"
)

// Estado de un módulo en una secuencia.
const (
	EstadoOK      = "success"
	EstadoError   = "error"
	EstadoOmitido = "omitido"
)

// Resultado de un módulo en una secuencia.
type Resultado struct {
	Modulo   string
	Estado   string
	Intentos int
	Mensaje  string
	Duracion time.Duration
}

// SecuenciaOpts son los parámetros de CorrerSecuencia (los cero toman el valor de Settings).
type SecuenciaOpts struct {
	Trigger     string // cli | cron | api
	Opciones    map[string]map[string]any
	Omitir      map[string]string
	Pausa       *time.Duration
	Reintentos  *int
	EsperaRein  *time.Duration
	EsperarLock time.Duration
	Sleep       func(time.Duration) // tests
	Alert       func(texto string)  // tests (por defecto, el webhook de Settings)
}

// CorrerSecuencia corre los módulos en orden bajo el lock global, uno detrás de otro (nunca en
// paralelo: Yahoo bloquea por ráfagas). Nunca falla por la caída de un módulo: devuelve el resultado
// de cada uno (core.ErrOcupado si no consigue el lock). Un pipeline que falla se reintenta y la
// secuencia sigue con el siguiente.
func CorrerSecuencia(ctx context.Context, env *Env, modulos []string, o SecuenciaOpts) ([]Resultado, error) {
	s := env.S
	if o.Trigger == "" {
		o.Trigger = "cli"
	}
	sleep := o.Sleep
	if sleep == nil {
		sleep = func(d time.Duration) {
			select {
			case <-time.After(d):
			case <-ctx.Done():
			}
		}
	}
	pausa := secs(s.PipelinePausaS)
	if o.Pausa != nil {
		pausa = *o.Pausa
	}
	maxRein := s.PipelineReintentos
	if o.Reintentos != nil {
		maxRein = *o.Reintentos
	}
	espera := secs(s.PipelineEsperaReintento)
	if o.EsperaRein != nil {
		espera = *o.EsperaRein
	}

	unlock, err := core.LockPipelines(s, o.EsperarLock)
	if err != nil {
		return nil, err
	}
	defer unlock()

	reg := core.NewJobRegistry(s.JobsDir())
	env.Log.Info("secuencia de pipelines", "modulos", strings.Join(modulos, " → "))
	pendientes := 0
	for _, m := range modulos {
		if _, omitido := o.Omitir[m]; !omitido {
			pendientes++
		}
	}
	var res []Resultado
	for _, m := range modulos {
		if why, omitido := o.Omitir[m]; omitido {
			env.Log.Info("omitido", "modulo", m, "motivo", why)
			res = append(res, Resultado{Modulo: m, Estado: EstadoOmitido, Mensaje: why})
			continue
		}
		r := correrModulo(ctx, env, reg, m, o.Trigger, o.Opciones[m], maxRein, espera, sleep)
		res = append(res, r)
		pendientes--
		if pendientes > 0 && pausa > 0 && ctx.Err() == nil {
			env.Log.Info("pausa antes del siguiente módulo (anti bloqueo de Yahoo)", "segundos", pausa.Seconds())
			sleep(pausa)
		}
	}

	var fallidos []Resultado
	for _, r := range res {
		if r.Estado == EstadoError {
			fallidos = append(fallidos, r)
		}
	}
	if len(fallidos) > 0 {
		alertar(env, fallidos, o.Trigger, o.Alert)
	}
	return res, nil
}

func secs(f float64) time.Duration { return time.Duration(f * float64(time.Second)) }

func correrModulo(ctx context.Context, env *Env, reg *core.JobRegistry, modulo, trigger string,
	opts map[string]any, maxRein int, espera time.Duration, sleep func(time.Duration)) Resultado {
	r := Resultado{Modulo: modulo, Estado: EstadoError}
	t0 := time.Now()
	defer func() { r.Duracion = time.Since(t0).Round(100 * time.Millisecond) }()

	p, ok := Get(modulo)
	if !ok {
		r.Mensaje = "el módulo no tiene pipeline portado"
		return r
	}
	if reg.IsRunning(modulo) {
		r.Mensaje = "ya había una corrida de este módulo en curso"
		env.Log.Warn("se omite", "modulo", modulo, "motivo", r.Mensaje)
		return r
	}
	for intento := 1; intento <= maxRein+1; intento++ {
		r.Intentos = intento
		env.Log.Info("inicio", "modulo", modulo, "intento", fmt.Sprintf("%d/%d", intento, maxRein+1), "opciones", opts)
		env.Traffic.ResetStats()
		rec, err := reg.Track(modulo, trigger, func(rec *core.JobRecord) error {
			msg, err := p.Run(ctx, env, opts)
			if msg != "" {
				rec.Message = &msg
			}
			if err == nil {
				if st := env.Traffic.Stats(); st.Requests > 0 {
					base := "OK"
					if msg != "" {
						base = msg
					}
					m := base + " · " + st.Summary()
					rec.Message = &m
				}
			}
			return err
		})
		if err != nil {
			r.Mensaje = err.Error()
			env.Log.Error("falló", "modulo", modulo, "error", r.Mensaje)
			if intento <= maxRein && ctx.Err() == nil {
				env.Log.Info("reintento", "modulo", modulo, "en_s", espera.Seconds())
				sleep(espera)
				continue
			}
			return r
		}
		r.Estado = EstadoOK
		if rec.Message != nil {
			r.Mensaje = *rec.Message
		}
		env.Log.Info("OK", "modulo", modulo, "mensaje", r.Mensaje)
		return r
	}
	return r
}

func alertar(env *Env, fallidos []Resultado, trigger string, custom func(string)) {
	partes := make([]string, len(fallidos))
	for i, f := range fallidos {
		partes[i] = fmt.Sprintf("%s (%s)", f.Modulo, f.Mensaje)
	}
	texto := fmt.Sprintf("Fog Radar Quant: falló la corrida (%s) de %s", trigger, strings.Join(partes, "; "))
	if custom != nil {
		custom(texto)
		return
	}
	url := env.S.AlertWebhookURL
	if url == "" {
		return
	}
	content := texto
	if len(content) > 1900 {
		content = content[:1900]
	}
	// «text» lo leen Slack/Teams y «content» Discord.
	body, _ := json.Marshal(map[string]string{"text": texto, "content": content})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		env.Log.Warn("no se pudo armar la alerta", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req) //nolint:gosec // URL configurada por el administrador
	if err != nil {
		env.Log.Warn("no se pudo enviar la alerta al webhook", "error", err) // el aviso nunca debe romper la corrida
		return
	}
	resp.Body.Close()
}

// Resumen formatea la tabla final de una secuencia.
func Resumen(res []Resultado) string {
	iconos := map[string]string{EstadoOK: "OK", EstadoError: "ERROR", EstadoOmitido: "omitido"}
	lineas := make([]string, len(res))
	for i, r := range res {
		lineas[i] = fmt.Sprintf("  %-10s %-8s %7.1fs  %s", r.Modulo, iconos[r.Estado], r.Duracion.Seconds(), r.Mensaje)
	}
	return strings.Join(lineas, "\n")
}
