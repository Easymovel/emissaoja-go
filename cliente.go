package emissaoja

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Versao é a versão desta biblioteca, enviada no cabeçalho User-Agent como emissaoja-go/<versão>.
const Versao = "0.1.0"

// Endereços da API. As chaves ej_live_ só valem em produção; as ej_test_, só no sandbox.
const (
	// BaseProducao é a API de produção, o padrão do cliente.
	BaseProducao = "https://emissaoja.com.br"
	// BaseSandbox é o ambiente de testes, com o Emissor Nacional simulado: nada vai ao fisco e nada é
	// cobrado.
	BaseSandbox = "https://sandbox.emissaoja.com.br"
)

// timeoutPadrao limita cada chamada HTTP quando ComTimeout não é usado.
const timeoutPadrao = 30 * time.Second

// Cliente fala com a API do emissaoja. Crie com [NovoCliente] e reaproveite a instância: ela é segura
// para uso concorrente. Cada recurso da API é um campo: c.Empresa, c.Tomadores, c.Servicos, c.Notas,
// c.Emissoes, c.Operacoes e c.CNPJ.
type Cliente struct {
	// Empresa conecta a empresa ao Emissor Nacional e responde as perguntas fiscais.
	Empresa *RecursoEmpresa
	// Tomadores cadastra e consulta os clientes que recebem as notas.
	Tomadores *RecursoTomadores
	// Servicos cadastra os serviços que a empresa presta e consulta o catálogo nacional.
	Servicos *RecursoServicos
	// Notas emite, acompanha, lista, baixa (PDF e XML) e cancela notas.
	Notas *RecursoNotas
	// Emissoes consulta e confere os pedidos de emissão.
	Emissoes *RecursoEmissoes
	// Operacoes acompanha as operações em segundo plano, como o cancelamento.
	Operacoes *RecursoOperacoes
	// CNPJ consulta os dados cadastrais de um CNPJ na Receita Federal.
	CNPJ *RecursoCNPJ

	baseURL     string
	httpCliente *http.Client
	timeout     time.Duration

	mu    sync.Mutex
	chave string

	// dormir e agora são trocados nos testes para não esperar de verdade.
	dormir func(ctx context.Context, d time.Duration) error
	agora  func() time.Time
}

// Opcao configura o [Cliente] em [NovoCliente].
type Opcao func(*Cliente)

// ComChave define a chave de API (ej_live_... em produção, ej_test_... no sandbox). Sem esta opção, ou
// com texto vazio, o cliente lê a variável de ambiente EMISSAOJA_KEY na primeira chamada que precisa da
// chave.
func ComChave(chave string) Opcao {
	chave = strings.TrimSpace(chave)
	return func(c *Cliente) {
		c.chave = chave
	}
}

// ComBaseURL troca o endereço da API, por exemplo para [BaseSandbox]. A barra no fim é ignorada; texto
// vazio mantém [BaseProducao].
func ComBaseURL(base string) Opcao {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	return func(c *Cliente) {
		if base != "" {
			c.baseURL = base
		}
	}
}

// ComHTTPClient usa o seu *http.Client (proxy, transporte próprio, testes). nil mantém o padrão.
func ComHTTPClient(h *http.Client) Opcao {
	return func(c *Cliente) {
		if h != nil {
			c.httpCliente = h
		}
	}
}

// ComTimeout limita cada chamada HTTP; o padrão é 30 s. Zero tira o limite do cliente, e vale só o
// prazo do contexto.
func ComTimeout(d time.Duration) Opcao {
	if d < 0 {
		d = 0
	}
	return func(c *Cliente) {
		c.timeout = d
	}
}

// NovoCliente cria o cliente. Sem opções, usa a API de produção, a chave da variável de ambiente
// EMISSAOJA_KEY e 30 s de limite por chamada.
func NovoCliente(opcoes ...Opcao) *Cliente {
	c := &Cliente{
		baseURL:     BaseProducao,
		httpCliente: &http.Client{},
		timeout:     timeoutPadrao,
		dormir:      dormirComContexto,
		agora:       time.Now,
	}
	for _, opcao := range opcoes {
		if opcao != nil {
			opcao(c)
		}
	}
	c.Empresa = &RecursoEmpresa{c: c}
	c.Tomadores = &RecursoTomadores{c: c}
	c.Servicos = &RecursoServicos{c: c}
	c.Notas = &RecursoNotas{c: c}
	c.Emissoes = &RecursoEmissoes{c: c}
	c.Operacoes = &RecursoOperacoes{c: c}
	c.CNPJ = &RecursoCNPJ{c: c}
	return c
}

// BaseURL devolve o endereço da API em uso, sem barra no fim.
func (c *Cliente) BaseURL() string {
	return c.baseURL
}

// Status consulta o estado do serviço (GET /v1/status): versão, motor e ambiente. Não exige chave.
func (c *Cliente) Status(ctx context.Context) (*StatusAPI, error) {
	r := get("/v1/status", nil)
	r.semChave = true
	return chamar[StatusAPI](ctx, c, r)
}

// chaveDaAPI devolve a chave. Sem ComChave, lê EMISSAOJA_KEY na primeira vez que ela é necessária.
func (c *Cliente) chaveDaAPI() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.chave == "" {
		c.chave = strings.TrimSpace(os.Getenv("EMISSAOJA_KEY"))
	}
	if c.chave == "" {
		return "", novoErro("chave_invalida", "Informe a chave de API: emissaoja.ComChave(...) ou a variável de ambiente EMISSAOJA_KEY.")
	}
	return c.chave, nil
}

// requisicao descreve uma chamada à API.
type requisicao struct {
	metodo     string
	caminho    string
	query      url.Values
	corpo      any
	cabecalhos map[string]string
	semChave   bool
}

// get monta um GET; q pode ser nil.
func get(caminho string, q url.Values) requisicao {
	return requisicao{metodo: http.MethodGet, caminho: caminho, query: q}
}

// comCorpo monta uma chamada com corpo JSON.
func comCorpo(metodo, caminho string, corpo any) requisicao {
	return requisicao{metodo: metodo, caminho: caminho, corpo: corpo}
}

// semCorpo monta uma chamada sem corpo (POST de ação, DELETE).
func semCorpo(metodo, caminho string) requisicao {
	return requisicao{metodo: metodo, caminho: caminho}
}

// chamar faz a chamada e decodifica a resposta JSON num T novo.
func chamar[T any](ctx context.Context, c *Cliente, r requisicao) (*T, error) {
	var saida T
	if err := c.chamarJSON(ctx, r, &saida); err != nil {
		return nil, err
	}
	return &saida, nil
}

// chamarMapa faz a chamada e devolve a resposta sem formato fixo (onboarding, CNPJ...).
func chamarMapa(ctx context.Context, c *Cliente, r requisicao) (map[string]any, error) {
	var saida map[string]any
	if err := c.chamarJSON(ctx, r, &saida); err != nil {
		return nil, err
	}
	if saida == nil {
		saida = map[string]any{}
	}
	return saida, nil
}

// chamarJSON faz a chamada e decodifica a resposta 2xx em saida.
func (c *Cliente) chamarJSON(ctx context.Context, r requisicao, saida any) error {
	status, cab, dados, err := c.enviar(ctx, r)
	if err != nil {
		return err
	}
	if status < 200 || status > 299 {
		return erroDaResposta(status, cab, dados)
	}
	if len(bytes.TrimSpace(dados)) == 0 {
		return nil
	}
	if err := json.Unmarshal(dados, saida); err != nil {
		return &Erro{Status: status, Code: "erro_desconhecido", Message: "Resposta da API fora do formato esperado: " + err.Error(), RequestID: cab.Get("X-Request-Id"), causa: err}
	}
	return nil
}

// chamarBytes faz um GET e devolve o corpo cru da resposta 2xx (PDF e XML).
func (c *Cliente) chamarBytes(ctx context.Context, caminho string) ([]byte, error) {
	status, cab, dados, err := c.enviar(ctx, get(caminho, nil))
	if err != nil {
		return nil, err
	}
	if status < 200 || status > 299 {
		return nil, erroDaResposta(status, cab, dados)
	}
	return dados, nil
}

// enviar faz a chamada HTTP e devolve status, cabeçalhos e corpo. O erro só aparece quando não houve
// resposta: chave ausente, corpo que não vira JSON, rede, prazo ou cancelamento.
func (c *Cliente) enviar(ctx context.Context, r requisicao) (int, http.Header, []byte, error) {
	ctx = contexto(ctx)
	var chave string
	if !r.semChave {
		k, err := c.chaveDaAPI()
		if err != nil {
			return 0, nil, nil, err
		}
		chave = k
	}

	endereco := c.baseURL + r.caminho
	if q := r.query.Encode(); q != "" {
		endereco += "?" + q
	}

	var corpo io.Reader
	if r.corpo != nil {
		dados, err := json.Marshal(r.corpo)
		if err != nil {
			return 0, nil, nil, &Erro{Code: "validation_error", Message: "Não foi possível montar o JSON do corpo: " + err.Error(), causa: err}
		}
		corpo = bytes.NewReader(dados)
	}

	ctxReq := ctx
	if c.timeout > 0 {
		var cancelar context.CancelFunc
		ctxReq, cancelar = context.WithTimeout(ctx, c.timeout)
		defer cancelar()
	}

	req, err := http.NewRequestWithContext(ctxReq, r.metodo, endereco, corpo)
	if err != nil {
		return 0, nil, nil, &Erro{Code: "rede", Message: "Endereço da API inválido: " + err.Error(), causa: err}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "emissaoja-go/"+Versao)
	for nome, valor := range r.cabecalhos {
		req.Header.Set(nome, valor)
	}
	if !r.semChave {
		req.Header.Set("Authorization", "Bearer "+chave)
	}
	if r.corpo != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpCliente.Do(req)
	if err != nil {
		return 0, nil, nil, erroDeFalha(ctx, ctxReq, err)
	}
	defer resp.Body.Close()
	dados, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, nil, erroDeFalha(ctx, ctxReq, err)
	}
	return resp.StatusCode, resp.Header, dados, nil
}

// mensagemTempo é a mensagem do erro tempo_esgotado de uma chamada.
const mensagemTempo = "A API não respondeu a tempo."

// erroDeFalha traduz uma falha sem resposta HTTP: contexto cancelado, prazo vencido ou rede.
func erroDeFalha(ctx, ctxReq context.Context, err error) *Erro {
	if errors.Is(ctx.Err(), context.Canceled) {
		return &Erro{Code: "cancelado", Message: "A chamada foi cancelada pelo contexto.", causa: context.Canceled}
	}
	if ctx.Err() != nil || ctxReq.Err() != nil {
		return &Erro{Code: "tempo_esgotado", Message: mensagemTempo, causa: context.DeadlineExceeded}
	}
	var falhaDeRede net.Error
	if errors.As(err, &falhaDeRede) && falhaDeRede.Timeout() {
		return &Erro{Code: "tempo_esgotado", Message: mensagemTempo, causa: err}
	}
	return &Erro{Code: "rede", Message: "Falha de rede: " + err.Error(), causa: err}
}

// erroDaResposta monta o *Erro de uma resposta fora de 2xx. Corpo JSON: code, message e o resto em
// Extras. Corpo que não é JSON: a mensagem é o texto, até 200 caracteres.
func erroDaResposta(status int, cab http.Header, dados []byte) *Erro {
	e := &Erro{Status: status, RequestID: cab.Get("X-Request-Id")}
	var corpo map[string]any
	if json.Unmarshal(dados, &corpo) == nil && corpo != nil {
		e.Code, _ = corpo["code"].(string)
		e.Message, _ = corpo["message"].(string)
		e.Extras = make(map[string]any, len(corpo))
		for nome, valor := range corpo {
			if nome != "status" && nome != "code" && nome != "message" {
				e.Extras[nome] = valor
			}
		}
	} else {
		e.Message = cortar(strings.TrimSpace(string(dados)), 200)
	}
	if e.Code == "" {
		e.Code = "erro_desconhecido"
	}
	if e.Message == "" {
		e.Message = fmt.Sprintf("HTTP %d", status)
	}
	return e
}

// cortar devolve no máximo n caracteres (runas) de s.
func cortar(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// definir põe o parâmetro na consulta só quando ele não está vazio.
func definir(q url.Values, nome, valor string) {
	if valor != "" {
		q.Set(nome, valor)
	}
}

// definirInteiro põe o parâmetro na consulta só quando ele não é zero.
func definirInteiro(q url.Values, nome string, valor int) {
	if valor != 0 {
		q.Set(nome, strconv.Itoa(valor))
	}
}

// segmento codifica um pedaço do caminho: barra, espaço, ? e # viram %XX.
func segmento(s string) string {
	return url.PathEscape(s)
}

// contexto troca um contexto nil por context.Background().
func contexto(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
