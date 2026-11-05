package emissaoja

import (
	"context"
	"time"
)

// AguardarOpcoes ajusta as esperas de [RecursoNotas.Aguardar], [RecursoOperacoes.Aguardar] e
// [RecursoEmpresa.AguardarCredencial]. Passe nil, ou deixe campos zerados, para usar o padrão de cada
// método. O prazo e o cancelamento do contexto também valem.
type AguardarOpcoes struct {
	// Timeout é o tempo máximo de espera. Padrão: 180 s (120 s em AguardarCredencial). Esgotado, o
	// método devolve o erro "tempo_esgotado".
	Timeout time.Duration
	// Intervalo é a pausa entre consultas. Em Notas.Aguardar é o mínimo: a pausa é o maior valor entre
	// Intervalo e o proxima_consulta_ms do pedido (3 s quando a API não informa). Padrão: 1 s em
	// Notas.Aguardar e 2 s nos demais.
	Intervalo time.Duration
}

// valores devolve o timeout e o intervalo efetivos, com os padrões do método no lugar dos zeros.
func (o *AguardarOpcoes) valores(timeoutDoMetodo, intervaloDoMetodo time.Duration) (time.Duration, time.Duration) {
	timeout, intervalo := timeoutDoMetodo, intervaloDoMetodo
	if o != nil {
		if o.Timeout > 0 {
			timeout = o.Timeout
		}
		if o.Intervalo > 0 {
			intervalo = o.Intervalo
		}
	}
	return timeout, intervalo
}

// dormirComContexto espera d ou até o contexto acabar, o que vier primeiro.
func dormirComContexto(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// esperar dorme entre duas consultas. Contexto cancelado ou vencido vira *Erro.
func (c *Cliente) esperar(ctx context.Context, d time.Duration) error {
	if err := c.dormir(ctx, d); err != nil {
		return erroDeFalha(ctx, ctx, err)
	}
	return nil
}
