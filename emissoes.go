package emissaoja

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// RecursoEmissoes reúne as rotas /v1/emissoes: os pedidos de emissão criados por [RecursoNotas.Emitir].
// Use pelo campo Cliente.Emissoes.
type RecursoEmissoes struct {
	c *Cliente
}

// FiltroEmissoes filtra [RecursoEmissoes.Listar].
type FiltroEmissoes struct {
	// Estados limita a lista a estes estados; vazio traz os pedidos em andamento (na_fila, emitindo,
	// confirmando e incerta).
	Estados []EstadoPedido
}

// Obter devolve o pedido de emissão (GET /v1/emissoes/{id}). Para esperar o fim, use
// [RecursoNotas.Aguardar].
func (r *RecursoEmissoes) Obter(ctx context.Context, id string) (*RespostaPedido, error) {
	return chamar[RespostaPedido](ctx, r.c, get("/v1/emissoes/"+segmento(id), nil))
}

// Listar devolve os pedidos de emissão (GET /v1/emissoes). filtro pode ser nil.
func (r *RecursoEmissoes) Listar(ctx context.Context, filtro *FiltroEmissoes) (*ListaPedidos, error) {
	q := url.Values{}
	if filtro != nil {
		estados := make([]string, 0, len(filtro.Estados))
		for _, e := range filtro.Estados {
			if e != "" {
				estados = append(estados, string(e))
			}
		}
		definir(q, "estados", strings.Join(estados, ","))
	}
	return chamar[ListaPedidos](ctx, r.c, get("/v1/emissoes", q))
}

// Conferir pede a conferência no portal de um pedido incerta (POST /v1/emissoes/{id}/conferir). A
// conferência segue em segundo plano; acompanhe com [RecursoEmissoes.Obter] ou [RecursoNotas.Aguardar].
// No sandbox, o valor 13.13 gera uma nota incerta que a conferência resolve como autorizada.
func (r *RecursoEmissoes) Conferir(ctx context.Context, id string) (*RespostaPedido, error) {
	return chamar[RespostaPedido](ctx, r.c, semCorpo(http.MethodPost, "/v1/emissoes/"+segmento(id)+"/conferir"))
}
