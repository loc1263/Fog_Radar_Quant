package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fogradar/internal/core"
	"fogradar/internal/retencion"
	"fogradar/internal/yahoo"
)

// Estado de un módulo en una secuencia.
const (
	EstadoOK      = "success"
	EstadoError   = "error"
	EstadoOmitido = "omitido"
)

// Resultado de un módulo en una secuencia.
type Resultado struct {
	Modulo  string
	Estado  string
	Mensaje string
	// Detalle es el mensaje del módulo sin el resumen de tráfico; Stats, el tráfico a Yahoo de su corrida.
	Detalle  string
	Stats    yahoo.Stats
	Duracion time.Duration
	Inicio   time.Time
	Fin      time.Time
	Avisos   []Aviso // WARN/ERROR que emitió el módulo durante su corrida
}

// SecuenciaOpts son los parámetros de CorrerSecuencia (los cero toman el valor de Settings).
type SecuenciaOpts struct {
	Opciones map[string]map[string]any
	Omitir   map[string]string
	Pausa    *time.Duration
	Sleep    func(time.Duration) // tests
}

// CorrerSecuencia corre los módulos en orden bajo el lock global, uno detrás de otro (nunca en
// paralelo: Yahoo bloquea por ráfagas). Nunca falla por la caída de un módulo: devuelve el resultado
// de cada uno (core.ErrOcupado si otra secuencia tiene el lock). Si un pipeline falla, la secuencia
// sigue con el siguiente.
func CorrerSecuencia(ctx context.Context, env *Env, modulos []string, o SecuenciaOpts) ([]Resultado, error) {
	s := env.S
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

	unlock, err := core.LockPipelines(s, 0)
	if err != nil {
		return nil, err
	}
	defer unlock()

	base := env.Log
	log, tomarAvisos := conAvisos(base)
	envC := *env // copia con el logger que anota los avisos de cada módulo
	envC.Log = log
	env = &envC
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
		t0 := time.Now()
		tomarAvisos() // descarta lo de antes (pausa, lock)
		r := correrModulo(ctx, env, reg, m, o.Opciones[m])
		r.Inicio, r.Fin = t0, time.Now()
		r.Duracion = r.Fin.Sub(t0).Round(100 * time.Millisecond)
		r.Avisos = tomarAvisos()
		if r.Estado == EstadoOK {
			podar(env, m)
		}
		res = append(res, r)
		pendientes--
		if pendientes > 0 && pausa > 0 && ctx.Err() == nil {
			if st := env.Traffic.Stats(); necesitaPausa(r, st) {
				env.Log.Info("pausa antes del siguiente módulo (anti bloqueo de Yahoo)", "segundos", pausa.Seconds(),
					"motivo", motivoPausa(r, st))
				sleep(pausa)
			} else {
				env.Log.Info("sin pausa antes del siguiente módulo: el anterior no recibió 429 (el espaciado global de Yahoo sigue activo)")
			}
		}
	}

	for _, r := range res {
		env.Log.Info("resultado", "modulo", r.Modulo, "estado", r.Estado, "duracion_s", r.Duracion.Seconds(), "mensaje", r.Mensaje)
	}
	return res, nil
}

// necesitaPausa dice si conviene esperar antes del siguiente módulo: solo si el anterior falló o Yahoo le respondió 429.
// Con tráfico limpio el espaciado global entre requests ya protege, y la pausa fija solo alargaba la secuencia.
// st son las estadísticas de la corrida del módulo.
func necesitaPausa(r Resultado, st yahoo.Stats) bool {
	return r.Estado == EstadoError || st.RateLimited > 0 || st.Retries > 0
}

func motivoPausa(r Resultado, st yahoo.Stats) string {
	if r.Estado == EstadoError {
		return "el módulo anterior falló"
	}
	return fmt.Sprintf("el módulo anterior recibió %d respuestas 429", st.RateLimited)
}

func secs(f float64) time.Duration { return time.Duration(f * float64(time.Second)) }

func correrModulo(ctx context.Context, env *Env, reg *core.JobRegistry, modulo string, opts map[string]any) Resultado {
	r := Resultado{Modulo: modulo, Estado: EstadoError}
	t0 := time.Now()

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
	env.Log.Info("inicio", "modulo", modulo, "opciones", opts)
	env.Traffic.ResetStats()
	var detalle string
	rec, err := reg.Track(modulo, func(rec *core.JobRecord) error {
		msg, err := p.Run(ctx, env, opts)
		detalle = msg
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
	r.Detalle, r.Stats = detalle, env.Traffic.Stats()
	if err != nil {
		r.Mensaje = err.Error()
		env.Log.Error("falló", "modulo", modulo, "duracion_s", time.Since(t0).Round(100*time.Millisecond).Seconds(), "error", r.Mensaje)
		return r
	}
	r.Estado = EstadoOK
	if rec.Message != nil {
		r.Mensaje = *rec.Message
	}
	env.Log.Info("OK", "modulo", modulo, "duracion_s", time.Since(t0).Round(100*time.Millisecond).Seconds(), "mensaje", r.Mensaje)
	return r
}

// podar borra de data/<modulo> lo que supera la retención (RETENCION_DIAS). Un fallo solo se avisa.
func podar(env *Env, modulo string) {
	if env.S.RetencionDias <= 0 {
		return
	}
	res, err := retencion.PodarModulo(env.S, modulo, env.S.RetencionDias, time.Now(), false)
	if err != nil {
		env.Log.Warn("no se pudo aplicar la retención de datos", "modulo", modulo, "error", err)
		return
	}
	if len(res.Eliminados) > 0 {
		env.Log.Info("retención: datos antiguos eliminados", "modulo", modulo, "dias", env.S.RetencionDias,
			"archivos", len(res.Eliminados), "mb", res.Bytes/(1<<20))
	}
}
