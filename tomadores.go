package emissaoja

import (
	"context"
	"net/http"
	"net/url"
)

// RecursoTomadores reúne as rotas /v1/tomadores: os clientes, pessoa jurídica ou física, que recebem as
// notas. Tomador não se apaga, porque as notas emitidas apontam para ele. Use pelo campo
// Cliente.Tomadores.
type RecursoTomadores struct {
	c *Cliente
}

// FiltroTomadores filtra [RecursoTomadores.Listar]. Campos vazios ficam fora da consulta.
type FiltroTomadores struct {
	// Busca filtra por nome ou documento.
	Busca string
	// Pagina começa em 1.
	Pagina int
	// Limite de tomadores por página, até 50.
	Limite int
	// Ordem é "nome" (padrão) ou "recentes".
	Ordem string
}

// Listar devolve uma página de tomadores (GET /v1/tomadores). filtro pode ser nil.
func (r *RecursoTomadores) Listar(ctx context.Context, filtro *FiltroTomadores) (*ListaTomadores, error) {
	q := url.Values{}
	if filtro != nil {
		definir(q, "busca", filtro.Busca)
		definirInteiro(q, "pagina", filtro.Pagina)
		definirInteiro(q, "limite", filtro.Limite)
		definir(q, "ordem", filtro.Ordem)
	}
	return chamar[ListaTomadores](ctx, r.c, get("/v1/tomadores", q))
}

// Buscar faz a busca rápida por nome ou documento, a partir de 2 caracteres (GET /v1/tomadores/busca).
func (r *RecursoTomadores) Buscar(ctx context.Context, q string) (*ListaTomadores, error) {
	consulta := url.Values{}
	definir(consulta, "q", q)
	return chamar[ListaTomadores](ctx, r.c, get("/v1/tomadores/busca", consulta))
}

// Criar cadastra um tomador (POST /v1/tomadores). Para CNPJ, passe só o documento e deixe nome vazio:
// nome e endereço vêm da Receita Federal. Para CPF, o nome completo é obrigatório. Se o tomador já
// existia, a API devolve o cadastro com JaExistia igual a true.
func (r *RecursoTomadores) Criar(ctx context.Context, documento, nome string) (*RespostaTomador, error) {
	corpo := map[string]any{"documento": documento}
	if nome != "" {
		corpo["nome"] = nome
	}
	return chamar[RespostaTomador](ctx, r.c, comCorpo(http.MethodPost, "/v1/tomadores", corpo))
}

// Obter devolve a ficha do tomador, com endereço e contato (GET /v1/tomadores/{tipo}/{id}).
func (r *RecursoTomadores) Obter(ctx context.Context, tipo TipoTomador, id string) (*RespostaTomador, error) {
	return chamar[RespostaTomador](ctx, r.c, get(caminhoTomador(tipo, id), nil))
}

// Atualizar edita endereço e contato do tomador (PUT /v1/tomadores/{tipo}/{id}); mande só o que muda.
// O documento é imutável. Chaves de texto (nil apaga o valor): nome, cep, endereco, numero,
// complemento, bairro, cidade, estado (a UF), codigo_municipio_ibge, telefone, email e
// inscricao_municipal. A chave atualizar_pela_receita recebe bool.
func (r *RecursoTomadores) Atualizar(ctx context.Context, tipo TipoTomador, id string, campos Campos) (*RespostaTomador, error) {
	if campos == nil {
		campos = Campos{}
	}
	return chamar[RespostaTomador](ctx, r.c, comCorpo(http.MethodPut, caminhoTomador(tipo, id), campos))
}

// caminhoTomador monta /v1/tomadores/{tipo}/{id} com os segmentos codificados.
func caminhoTomador(tipo TipoTomador, id string) string {
	return "/v1/tomadores/" + segmento(string(tipo)) + "/" + segmento(id)
}
