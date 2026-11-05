package emissaoja

import (
	"context"
	"net/http"
	"time"
)

// RecursoEmpresa reúne as rotas /v1/empresa: conectar a empresa ao Emissor Nacional com o CNPJ e a
// senha do portal e responder as perguntas fiscais. Uma conta tem uma empresa. Use pelo campo
// Cliente.Empresa.
type RecursoEmpresa struct {
	c *Cliente
}

// CamposConexao é o corpo de [RecursoEmpresa.Conectar].
type CamposConexao struct {
	// CNPJ da empresa, só com números. O CNPJ alfanumérico ainda não é aceito.
	CNPJ string `json:"cnpj"`
	// Senha do Emissor Nacional (nfse.gov.br). Ela nunca volta nas respostas da API.
	Senha string `json:"senha"`
	// CienteUltimaTentativa confirma a última tentativa de senha do dia para este CNPJ, pedida pelo erro
	// "confirmar_ultima_tentativa". Vai sempre no corpo.
	CienteUltimaTentativa bool `json:"ciente_ultima_tentativa"`
}

// CamposFiscais é o corpo de [RecursoEmpresa.Fiscal]: as perguntas que o cadastro fiscal não responde
// sozinho. Campos zerados ficam fora do corpo.
type CamposFiscais struct {
	// AliquotaSimples é a alíquota do Simples Nacional em %, de 0,01 a 33, pedida a ME e EPP.
	AliquotaSimples float64 `json:"aliquota_simples,omitempty"`
	// RegApTribSN é o regime de apuração no Simples: 1 (tributos federais e municipal pelo Simples
	// Nacional), 2 (federais pelo Simples; ISS pela regra do município) ou 3 (federais e municipal fora
	// do Simples Nacional).
	RegApTribSN int `json:"reg_ap_trib_sn,omitempty"`
}

// Obter devolve a empresa da conta com o estado da credencial, do onboarding e da importação
// (GET /v1/empresa).
func (r *RecursoEmpresa) Obter(ctx context.Context) (*RespostaEmpresa, error) {
	return chamar[RespostaEmpresa](ctx, r.c, get("/v1/empresa", nil))
}

// Conectar envia o CNPJ e a senha do Emissor Nacional (PUT /v1/empresa/credencial). A API devolve a
// tentativa (202) e confere a senha no portal em segundo plano; depois lê o cadastro fiscal e traz as
// notas dos últimos 90 dias. Acompanhe com [RecursoEmpresa.AguardarCredencial].
//
// O portal bloqueia a conta da empresa depois de algumas senhas erradas, por isso a API aceita no
// máximo 3 falhas por CNPJ em 24 horas.
func (r *RecursoEmpresa) Conectar(ctx context.Context, campos CamposConexao) (*RespostaConexao, error) {
	return chamar[RespostaConexao](ctx, r.c, comCorpo(http.MethodPut, "/v1/empresa/credencial", campos))
}

// Credencial devolve o estado da credencial e a verificação em andamento, ou a última
// (GET /v1/empresa/credencial).
func (r *RecursoEmpresa) Credencial(ctx context.Context) (*RespostaCredencial, error) {
	return chamar[RespostaCredencial](ctx, r.c, get("/v1/empresa/credencial", nil))
}

// AguardarCredencial consulta a credencial até a tentativa sair de "aguardar" (ou não haver tentativa)
// e devolve a última resposta; veja Tentativa.ProximoPasso: "pronto", "configurando", "corrigir_senha",
// "tentar_mais_tarde", "redefinir_no_portal" ou "suporte". Padrão: uma consulta a cada 2 s, por até
// 120 s. opcoes pode ser nil.
func (r *RecursoEmpresa) AguardarCredencial(ctx context.Context, opcoes *AguardarOpcoes) (*RespostaCredencial, error) {
	ctx = contexto(ctx)
	timeout, intervalo := opcoes.valores(120*time.Second, 2*time.Second)
	fim := r.c.agora().Add(timeout)
	for {
		resp, err := r.Credencial(ctx)
		if err != nil {
			return nil, err
		}
		if resp.Tentativa == nil || resp.Tentativa.ProximoPasso != "aguardar" {
			return resp, nil
		}
		if r.c.agora().After(fim) {
			return nil, novoErro("tempo_esgotado", "A verificação da senha não terminou a tempo.")
		}
		if err := r.c.esperar(ctx, intervalo); err != nil {
			return nil, err
		}
	}
}

// Onboarding devolve os estágios, as perguntas pendentes e o bloqueio da configuração da empresa
// (GET /v1/empresa/onboarding), em formato livre.
func (r *RecursoEmpresa) Onboarding(ctx context.Context) (map[string]any, error) {
	return chamarMapa(ctx, r.c, get("/v1/empresa/onboarding", nil))
}

// Fiscal responde as perguntas que o cadastro fiscal não responde sozinho, como a alíquota do Simples
// de ME e EPP (PUT /v1/empresa/fiscal).
func (r *RecursoEmpresa) Fiscal(ctx context.Context, campos CamposFiscais) (map[string]any, error) {
	return chamarMapa(ctx, r.c, comCorpo(http.MethodPut, "/v1/empresa/fiscal", campos))
}

// RelerFisco pede uma nova leitura do cadastro fiscal no portal (POST /v1/empresa/reler-fisco).
func (r *RecursoEmpresa) RelerFisco(ctx context.Context) (map[string]any, error) {
	return chamarMapa(ctx, r.c, semCorpo(http.MethodPost, "/v1/empresa/reler-fisco"))
}
