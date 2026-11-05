package emissaoja

import (
	"context"
	"net/http"
	"net/url"
)

// RecursoServicos reúne as rotas /v1/servicos, os serviços que a empresa presta, e o catálogo nacional
// de códigos de serviço. Use pelo campo Cliente.Servicos.
type RecursoServicos struct {
	c *Cliente
}

// FiltroServicos filtra [RecursoServicos.Listar].
type FiltroServicos struct {
	// Ativos é "1" (só os ativos, o padrão da API), "0" (só os desativados) ou "todos".
	Ativos string
}

// CamposServico é o corpo de [RecursoServicos.Criar]. Descricao, CTribNac, TribISSQN e RetemISS são
// obrigatórios; os demais ficam fora do corpo quando vazios.
type CamposServico struct {
	// Descricao é o nome do serviço.
	Descricao string `json:"descricao"`
	// CTribNac é o código de tributação nacional, com 6 dígitos; ache o seu em RecursoServicos.Catalogo.
	CTribNac string `json:"c_trib_nac"`
	// TribISSQN é "tributavel" ou "nao_incidencia". Isenção, imunidade e exportação não são aceitas.
	TribISSQN string `json:"trib_issqn"`
	// RetemISS diz se o tomador retém o ISS. Vai sempre no corpo, inclusive false.
	RetemISS bool `json:"retem_iss"`
	// NBS é o código NBS, quando o item do catálogo pede.
	NBS string `json:"nbs,omitempty"`
	// CTribMun é o código do serviço na prefeitura (3 dígitos), quando ela exige.
	CTribMun string `json:"c_trib_mun,omitempty"`
	// TpOper é o tipo de operação, de 1 a 5: quando o pagamento acontece em relação à prestação.
	TpOper int `json:"tp_oper,omitempty"`
	// AliquotaISS é a alíquota do ISS em %, quando pedida.
	AliquotaISS float64 `json:"aliquota_iss,omitempty"`
	// DescricaoDaNota é a descrição que vai na nota, quando difere da Descricao.
	DescricaoDaNota string `json:"descricao_da_nota,omitempty"`
	// Principal marca o serviço usado quando a nota não informa ServicoID.
	Principal bool `json:"principal,omitempty"`
}

// Listar devolve os serviços cadastrados (GET /v1/servicos). filtro pode ser nil.
func (r *RecursoServicos) Listar(ctx context.Context, filtro *FiltroServicos) (*ListaServicos, error) {
	q := url.Values{}
	if filtro != nil {
		definir(q, "ativos", filtro.Ativos)
	}
	return chamar[ListaServicos](ctx, r.c, get("/v1/servicos", q))
}

// Criar cadastra um serviço (POST /v1/servicos). Ache o CTribNac em [RecursoServicos.Catalogo].
func (r *RecursoServicos) Criar(ctx context.Context, campos CamposServico) (*RespostaServico, error) {
	return chamar[RespostaServico](ctx, r.c, comCorpo(http.MethodPost, "/v1/servicos", campos))
}

// Atualizar edita o serviço (PUT /v1/servicos/{id}); mande só o que muda, com os nomes da API:
// descricao, descricao_da_nota, principal, c_trib_nac, c_trib_mun, nbs, trib_issqn, tp_oper, retem_iss
// e aliquota_iss.
func (r *RecursoServicos) Atualizar(ctx context.Context, id string, campos Campos) (*RespostaServico, error) {
	if campos == nil {
		campos = Campos{}
	}
	return chamar[RespostaServico](ctx, r.c, comCorpo(http.MethodPut, "/v1/servicos/"+segmento(id), campos))
}

// Desativar desativa o serviço (DELETE /v1/servicos/{id}); as notas emitidas continuam apontando para
// ele. O último serviço ativo não pode ser desativado (erro "ultimo_servico_ativo").
func (r *RecursoServicos) Desativar(ctx context.Context, id string) (*RespostaServico, error) {
	return chamar[RespostaServico](ctx, r.c, semCorpo(http.MethodDelete, "/v1/servicos/"+segmento(id)))
}

// Catalogo busca códigos no catálogo nacional de serviços por texto (GET /v1/catalogo/servicos). O
// limite vai até 50; zero usa o padrão da API.
func (r *RecursoServicos) Catalogo(ctx context.Context, q string, limite int) (*ListaCatalogo, error) {
	consulta := url.Values{}
	definir(consulta, "q", q)
	definirInteiro(consulta, "limite", limite)
	return chamar[ListaCatalogo](ctx, r.c, get("/v1/catalogo/servicos", consulta))
}
