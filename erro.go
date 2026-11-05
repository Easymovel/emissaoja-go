package emissaoja

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Erro é o erro devolvido por todos os métodos do cliente. Respostas fora de 2xx viram Erro com o Code
// da lista fechada da API; falhas antes da resposta também, com Status 0:
//
//   - "rede": a chamada não chegou à API ou a conexão caiu.
//   - "tempo_esgotado": venceu o prazo da chamada ([ComTimeout]), do contexto ou de um Aguardar.
//   - "cancelado": o contexto foi cancelado.
//   - "chave_invalida": não há chave ([ComChave] nem EMISSAOJA_KEY).
//   - "chave_idempotencia_invalida": a referência de [RecursoNotas.Emitir] está fora do formato.
//
// Use errors.As para chegar ao Erro, ou [CodigoDoErro] para ler só o Code. Em falhas de contexto,
// errors.Is(err, context.Canceled) e errors.Is(err, context.DeadlineExceeded) também funcionam.
type Erro struct {
	// Status é o código HTTP da resposta, ou 0 quando o erro nasceu no cliente.
	Status int `json:"status"`
	// Code é o código da lista fechada da API, como "nota_bloqueada" ou "limite_notas". Decida por ele,
	// nunca pela mensagem. Sem code no corpo da resposta, vale "erro_desconhecido".
	Code string `json:"code"`
	// Message é a mensagem em português: a do corpo da resposta ou "HTTP <status>".
	Message string `json:"message"`
	// Extras são os outros campos do corpo do erro, como bloqueios, nota_igual_recente, pedido_id,
	// retry_after e details. Números chegam como float64, como em encoding/json.
	Extras map[string]any `json:"extras,omitempty"`
	// RequestID é o cabeçalho X-Request-Id da resposta. Informe-o ao suporte.
	RequestID string `json:"request_id,omitempty"`

	causa error
}

// Error devolve "emissaoja: <code> (HTTP <status>): <mensagem>", com o request id quando houver.
func (e *Erro) Error() string {
	s := "emissaoja: " + e.Code
	if e.Status > 0 {
		s += fmt.Sprintf(" (HTTP %d)", e.Status)
	}
	s += ": " + e.Message
	if e.RequestID != "" {
		s += " [request_id " + e.RequestID + "]"
	}
	return s
}

// Unwrap devolve a causa de um erro nascido no cliente, como context.Canceled ou a falha de rede.
func (e *Erro) Unwrap() error {
	return e.causa
}

// Bloqueio é um motivo que impede a nota de sair (erro "nota_bloqueada"): o code, a mensagem em
// português e a ação que resolve, como "completar_tomador" ou "responder_aliquota".
type Bloqueio struct {
	// Code do bloqueio.
	Code string `json:"code"`
	// Mensagem em português, para mostrar a quem emite.
	Mensagem string `json:"mensagem"`
	// Acao é o que resolve o bloqueio.
	Acao string `json:"acao"`
}

// Bloqueios devolve os bloqueios do erro "nota_bloqueada" (Extras["bloqueios"]), ou nil quando não há.
func (e *Erro) Bloqueios() []Bloqueio {
	if e == nil {
		return nil
	}
	bruto, ok := e.Extras["bloqueios"]
	if !ok || bruto == nil {
		return nil
	}
	dados, err := json.Marshal(bruto)
	if err != nil {
		return nil
	}
	var lista []Bloqueio
	if json.Unmarshal(dados, &lista) != nil {
		return nil
	}
	return lista
}

// CodigoDoErro devolve o Code de um *[Erro], mesmo embrulhado com %w, ou "" para nil e outros erros.
func CodigoDoErro(err error) string {
	var e *Erro
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// novoErro cria um erro nascido no cliente (Status 0).
func novoErro(code, mensagem string) *Erro {
	return &Erro{Code: code, Message: mensagem}
}
