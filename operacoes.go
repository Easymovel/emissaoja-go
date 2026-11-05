package emissaoja

import (
	"context"
	"time"
)

// RecursoOperacoes acompanha as operações em segundo plano, como o cancelamento de uma nota. Use pelo
// campo Cliente.Operacoes.
type RecursoOperacoes struct {
	c *Cliente
}

// Obter devolve a operação (GET /v1/operacoes/{id}).
func (r *RecursoOperacoes) Obter(ctx context.Context, id string) (*RespostaOperacao, error) {
	return chamar[RespostaOperacao](ctx, r.c, get("/v1/operacoes/"+segmento(id), nil))
}

// Aguardar consulta a operação até ela sair de "processando" (concluida ou falhou) e devolve a última
// resposta. Padrão: uma consulta a cada 2 s, por até 180 s; esgotado o prazo, devolve o erro
// "tempo_esgotado". opcoes pode ser nil.
func (r *RecursoOperacoes) Aguardar(ctx context.Context, id string, opcoes *AguardarOpcoes) (*RespostaOperacao, error) {
	ctx = contexto(ctx)
	timeout, intervalo := opcoes.valores(180*time.Second, 2*time.Second)
	fim := r.c.agora().Add(timeout)
	for {
		resp, err := r.Obter(ctx, id)
		if err != nil {
			return nil, err
		}
		if resp.Operacao.Estado != "processando" {
			return resp, nil
		}
		if r.c.agora().After(fim) {
			return nil, novoErro("tempo_esgotado", "A operação não terminou a tempo.")
		}
		if err := r.c.esperar(ctx, intervalo); err != nil {
			return nil, err
		}
	}
}
