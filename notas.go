package emissaoja

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

// reReferencia é o formato da referência (Idempotency-Key) que a API aceita.
var reReferencia = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

// RecursoNotas reúne as rotas /v1/notas: emitir, acompanhar, listar, baixar o PDF e o XML e cancelar.
// Use pelo campo Cliente.Notas.
type RecursoNotas struct {
	c *Cliente
}

// CamposNota é o corpo de [RecursoNotas.Emitir]. TomadorTipo, TomadorID e Valor são obrigatórios; os
// demais ficam fora do corpo quando vazios.
type CamposNota struct {
	// TomadorTipo é TomadorPJ ou TomadorPF.
	TomadorTipo TipoTomador `json:"tomador_tipo"`
	// TomadorID é o id do tomador (RespostaTomador.Tomador.ID).
	TomadorID string `json:"tomador_id"`
	// Valor da nota em reais. A API aceita até 2 casas decimais: 1500, 150.5 e 99.90 servem; 10.005, não.
	Valor float64 `json:"valor"`
	// ServicoID escolhe o serviço; vazio usa o serviço principal.
	ServicoID string `json:"servico_id,omitempty"`
	// Descricao é o texto da nota, até 2000 caracteres; vazio usa a descrição do serviço.
	Descricao string `json:"descricao,omitempty"`
	// Competencia é a data do serviço (AAAA-MM-DD); vazio usa hoje.
	Competencia string `json:"competencia,omitempty"`
	// RetemISS só quando um bloqueio pedir: diz se o tomador retém o ISS. Use Ptr(true) ou Ptr(false).
	RetemISS *bool `json:"retem_iss,omitempty"`
	// AliquotaISS é a alíquota do ISS em %, só quando um bloqueio pedir.
	AliquotaISS float64 `json:"aliquota_iss,omitempty"`
	// MunicipioPrestacao é o código IBGE da cidade onde o serviço foi feito, só quando um bloqueio pedir.
	MunicipioPrestacao string `json:"municipio_prestacao,omitempty"`
	// ConfirmoRepeticao confirma a mesma nota emitida há menos de 2 minutos com outra referência (erro
	// "emissao_repetida_recente").
	ConfirmoRepeticao bool `json:"confirmo_repeticao,omitempty"`
	// PermitirRepetidaDe confirma uma nota igual a outra de hoje (erro "possivel_duplicada"): informe o
	// pedido_id de Extras["nota_igual_recente"].
	PermitirRepetidaDe string `json:"permitir_repetida_de,omitempty"`
}

// FiltroNotas filtra [RecursoNotas.Listar]. Campos vazios ficam fora da consulta.
type FiltroNotas struct {
	// Periodo é "mes_atual", "mes_anterior", "ano_atual", "12_meses" (padrão) ou "personalizado", com
	// De e Ate.
	Periodo string
	// De é a data inicial (AAAA-MM-DD) do período personalizado.
	De string
	// Ate é a data final (AAAA-MM-DD) do período personalizado.
	Ate string
	// Situacao é "todas" (padrão), "emitidas" ou "canceladas".
	Situacao string
	// Busca filtra pelo número da nota ou pelo nome ou documento do tomador.
	Busca string
	// Cursor é o ListaNotas.ProximoCursor da página anterior.
	Cursor string
	// Limite de notas por página, até 50 (padrão 20).
	Limite int
}

// Emitir pede a emissão de uma NFS-e (POST /v1/notas): prévia e emissão num passo só.
//
// A referência é um id seu para esta nota (8 a 64 caracteres: letras, números, _ e -), enviado no
// cabeçalho Idempotency-Key. Repetir a mesma referência devolve o mesmo pedido, com Repetido igual a
// true, e nunca emite duas vezes. Referência fora do formato devolve o erro
// "chave_idempotencia_invalida" sem chamar a API.
//
// A resposta traz o pedido na fila; acompanhe com [RecursoNotas.Aguardar]. Erros comuns:
// "nota_bloqueada" (veja [Erro.Bloqueios]), "possivel_duplicada" (repita com PermitirRepetidaDe),
// "emissao_repetida_recente" (repita com ConfirmoRepeticao), "limite_notas" (limite do plano no mês) e
// "fila_indisponivel" (repita com a mesma referência).
func (r *RecursoNotas) Emitir(ctx context.Context, referencia string, campos CamposNota) (*RespostaEmissao, error) {
	if !reReferencia.MatchString(referencia) {
		return nil, novoErro("chave_idempotencia_invalida", "Informe a referência da nota: 8 a 64 caracteres (letras, números, _ e -), única por nota.")
	}
	req := comCorpo(http.MethodPost, "/v1/notas", campos)
	req.cabecalhos = map[string]string{"Idempotency-Key": referencia}
	return chamar[RespostaEmissao](ctx, r.c, req)
}

// Aguardar consulta o pedido (GET /v1/emissoes/{id}) até autorizada, recusada ou incerta e devolve a
// última resposta. Entre as consultas, espera o proxima_consulta_ms que a API devolve (3 s quando ela
// não informa), nunca menos que opcoes.Intervalo (padrão 1 s). O prazo padrão é 180 s; esgotado, o
// método devolve o erro "tempo_esgotado" com o pedido_id em Extras. Cancelar o contexto interrompe a
// espera. opcoes pode ser nil.
func (r *RecursoNotas) Aguardar(ctx context.Context, pedidoID string, opcoes *AguardarOpcoes) (*RespostaPedido, error) {
	ctx = contexto(ctx)
	timeout, minimo := opcoes.valores(180*time.Second, time.Second)
	fim := r.c.agora().Add(timeout)
	for {
		resp, err := r.c.Emissoes.Obter(ctx, pedidoID)
		if err != nil {
			return nil, err
		}
		if resp.Pedido.Estado.Terminal() {
			return resp, nil
		}
		if r.c.agora().After(fim) {
			e := novoErro("tempo_esgotado", "O pedido não terminou a tempo; consulte de novo mais tarde.")
			e.Extras = map[string]any{"pedido_id": pedidoID}
			return nil, e
		}
		espera := 3 * time.Second
		if p := resp.Pedido.ProximaConsultaMs; p != nil && *p > 0 {
			espera = time.Duration(*p) * time.Millisecond
		}
		if espera < minimo {
			espera = minimo
		}
		if err := r.c.esperar(ctx, espera); err != nil {
			return nil, err
		}
	}
}

// Listar devolve as notas emitidas pelo emissaoja e as importadas do portal (GET /v1/notas), paginadas
// por cursor. filtro pode ser nil.
func (r *RecursoNotas) Listar(ctx context.Context, filtro *FiltroNotas) (*ListaNotas, error) {
	q := url.Values{}
	if filtro != nil {
		definir(q, "periodo", filtro.Periodo)
		definir(q, "de", filtro.De)
		definir(q, "ate", filtro.Ate)
		definir(q, "situacao", filtro.Situacao)
		definir(q, "busca", filtro.Busca)
		definir(q, "cursor", filtro.Cursor)
		definirInteiro(q, "limite", filtro.Limite)
	}
	return chamar[ListaNotas](ctx, r.c, get("/v1/notas", q))
}

// PDF baixa o DANFSe da nota, em PDF (GET /v1/notas/{origem}/{id}/pdf).
func (r *RecursoNotas) PDF(ctx context.Context, origem OrigemNota, id string) ([]byte, error) {
	return r.c.chamarBytes(ctx, caminhoNota(origem, id, "pdf"))
}

// XML baixa o XML da nota, em UTF-8 (GET /v1/notas/{origem}/{id}/xml).
func (r *RecursoNotas) XML(ctx context.Context, origem OrigemNota, id string) (string, error) {
	dados, err := r.c.chamarBytes(ctx, caminhoNota(origem, id, "xml"))
	if err != nil {
		return "", err
	}
	return string(dados), nil
}

// Cancelar pede o cancelamento da nota dentro do prazo do portal (POST /v1/notas/{origem}/{id}/cancelar).
// A justificativa é opcional: de 15 a 255 caracteres, e vazia fica fora do corpo. A API devolve a
// operação (202), que segue em segundo plano; acompanhe com [RecursoOperacoes.Aguardar]. Nota não se
// altera: para corrigir, cancele e emita outra. Nota autorizada e depois cancelada conta no plano.
func (r *RecursoNotas) Cancelar(ctx context.Context, origem OrigemNota, id string, motivo MotivoCancelamento, justificativa string) (*RespostaOperacao, error) {
	corpo := map[string]any{"motivo": string(motivo)}
	if justificativa != "" {
		corpo["justificativa"] = justificativa
	}
	return chamar[RespostaOperacao](ctx, r.c, comCorpo(http.MethodPost, caminhoNota(origem, id, "cancelar"), corpo))
}

// caminhoNota monta /v1/notas/{origem}/{id}/{acao} com os segmentos codificados.
func caminhoNota(origem OrigemNota, id, acao string) string {
	return "/v1/notas/" + segmento(string(origem)) + "/" + segmento(id) + "/" + acao
}
