# emissaoja: API de NFS-e nacional para Go

[![Go Reference](https://pkg.go.dev/badge/emissaoja.com.br/go.svg)](https://pkg.go.dev/emissaoja.com.br/go) [![CI](https://github.com/Easymovel/emissaoja-go/actions/workflows/ci.yml/badge.svg)](https://github.com/Easymovel/emissaoja-go/actions/workflows/ci.yml) [![Licença MIT](https://img.shields.io/badge/licen%C3%A7a-MIT-blue)](LICENSE)

Cliente oficial em Go da **[API do emissaoja](https://emissaoja.com.br/docs/)** para emitir **NFS-e (Nota Fiscal de Serviço eletrônica) no Emissor Nacional** (nfse.gov.br) a partir do seu sistema, **sem certificado digital**: o emissaoja entra no portal com o CNPJ e a senha da própria empresa. Feito para **MEI e empresas do Simples Nacional**. Cadastre tomadores pelo CNPJ (nome e endereço vêm da Receita Federal), emita, acompanhe a autorização, baixe o **PDF (DANFSe) e o XML** e cancele notas.

- **Zero dependências:** só a biblioteca padrão, Go 1.21 ou mais novo.
- **Tipado:** `Pedido`, `Nota`, `Tomador`, `Servico` e `Operacao` com tags JSON; `context.Context` em todas as chamadas; cliente seguro para uso concorrente.
- **Idempotência embutida:** toda emissão leva uma referência sua, que vira o cabeçalho `Idempotency-Key`. Repetir a mesma referência nunca emite duas notas.
- `Notas.Aguardar` acompanha o pedido até `autorizada`, `recusada` ou `incerta`, no ritmo que a API indica (`proxima_consulta_ms`), e para quando o contexto é cancelado.
- Erros como `*emissaoja.Erro`, com o `code` estável da API (`nota_bloqueada`, `limite_notas`, `possivel_duplicada`…), os bloqueios a resolver e o `request_id`.
- **Sandbox grátis** com Emissor Nacional simulado: teste o fluxo inteiro sem efeito fiscal.

## Instalação

```bash
go get emissaoja.com.br/go
```

O caminho de import termina em `go`, mas o pacote se chama `emissaoja`. Importe com o nome explícito:

```go
import emissaoja "emissaoja.com.br/go"
```

Crie a conta com o Google em [emissaoja.com.br](https://emissaoja.com.br/?origem=lib-go) e gere a chave em **Conta → API** (ela aparece uma vez; prefixo `ej_live_`). Guarde a chave no servidor:

```bash
export EMISSAOJA_KEY=ej_live_...
```

## Teste no sandbox em 2 minutos

O sandbox (`https://sandbox.emissaoja.com.br`) é o mesmo emissaoja com um **Emissor Nacional simulado**: nada ali existe no fisco e nada é cobrado. Entre com o Google, crie uma chave `ej_test_` em Conta → API e rode este `main.go` com `EMISSAOJA_KEY=ej_test_... go run .`:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	emissaoja "emissaoja.com.br/go"
)

func main() {
	ctx := context.Background()
	c := emissaoja.NovoCliente(emissaoja.ComBaseURL(emissaoja.BaseSandbox)) // chave em EMISSAOJA_KEY

	// 1. Conectar a empresa (uma vez). 11111111000191 é um MEI de exemplo pronto para emitir; qualquer senha serve.
	if _, err := c.Empresa.Conectar(ctx, emissaoja.CamposConexao{CNPJ: "11111111000191", Senha: "senha-de-teste"}); err != nil {
		log.Fatal(err)
	}
	if _, err := c.Empresa.AguardarCredencial(ctx, nil); err != nil {
		log.Fatal(err)
	}

	// 2. Tomador pelo CNPJ: nome e endereço vêm da Receita Federal.
	t, err := c.Tomadores.Criar(ctx, "12345678000195", "")
	if err != nil {
		log.Fatal(err)
	}

	// 3. Emitir (prévia + emissão num passo). A referência é o seu id da nota.
	r, err := c.Notas.Emitir(ctx, "pedido-2026-11-0001", emissaoja.CamposNota{
		TomadorTipo: t.Tomador.Tipo,
		TomadorID:   t.Tomador.ID,
		Valor:       1500,
		Descricao:   "Consultoria em marketing digital - novembro/2026",
	})
	if err != nil {
		log.Fatal(err)
	}

	// 4. Acompanhar até autorizada | recusada | incerta.
	fim, err := c.Notas.Aguardar(ctx, r.Pedido.ID, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(fim.Pedido.Estado, fim.Pedido.Numero, fim.Pedido.ChaveAcesso)

	// 5. PDF (DANFSe) e XML.
	if fim.Pedido.Estado == emissaoja.EstadoAutorizada {
		pdf, err := c.Notas.PDF(ctx, fim.Pedido.Nota.Origem, fim.Pedido.Nota.ID)
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile("nfse.pdf", pdf, 0o600); err != nil {
			log.Fatal(err)
		}
		xml, err := c.Notas.XML(ctx, fim.Pedido.Nota.Origem, fim.Pedido.Nota.ID)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(len(xml), "caracteres de XML")
	}
}
```

Valores com significado no sandbox: `13.13` gera uma nota **incerta** que `c.Emissoes.Conferir(ctx, id)` resolve como autorizada, `77.77` uma incerta que não saiu, `66.66` uma **recusada** pelo fisco e `99.99` um **bloqueio** já na prévia (`422 nota_bloqueada`). Outras empresas de exemplo: `22222222000191` (ME que pede a alíquota do Simples antes de emitir), `33333333000191` (MEI sem notas anteriores) e `44444444000191` (bloqueada). A senha `errada` simula senha recusada pelo portal. Tomadores de exemplo: CNPJ `12345678000195` e CPF `52998224725`. O mesmo fluxo está no [exemplo do pacote no pkg.go.dev](https://pkg.go.dev/emissaoja.com.br/go#example-package).

## Em produção

`emissaoja.NovoCliente()` usa `https://emissaoja.com.br` (`emissaoja.BaseProducao`) e lê `EMISSAOJA_KEY` na primeira chamada que precisa da chave. A chave `ej_live_` vale para a empresa e o plano da conta que a criou; chaves `ej_test_` só valem no sandbox.

> **A chave é secreta.** Use o cliente só no servidor (APIs, filas, jobs, CLIs internas). Quem tiver a chave emite notas em nome da empresa: nunca a coloque em navegador, app de celular ou binário distribuído.

Crie um cliente por processo e reaproveite: ele é seguro para uso concorrente e não guarda estado entre chamadas além da chave.

```go
var ej = emissaoja.NovoCliente(
	emissaoja.ComChave(os.Getenv("MINHA_CHAVE_EMISSAOJA")),          // sem ComChave: lê EMISSAOJA_KEY
	emissaoja.ComTimeout(15*time.Second),                            // padrão: 30 s por chamada
	emissaoja.ComHTTPClient(&http.Client{Transport: meuTransporte}), // proxy, observabilidade, testes
)
```

Todos os métodos recebem um `context.Context` primeiro: prazo e cancelamento do contexto valem para a chamada e para as esperas de `Aguardar`.

## Empresa: conectar ao Emissor Nacional

```go
// 202: a verificação da senha segue em segundo plano
_, err := c.Empresa.Conectar(ctx, emissaoja.CamposConexao{CNPJ: "11222333000181", Senha: "senha-do-emissor-nacional"})

cred, err := c.Empresa.AguardarCredencial(ctx, nil)
// cred.Tentativa.ProximoPasso: configurando | pronto | corrigir_senha | tentar_mais_tarde | redefinir_no_portal | suporte

emp, err := c.Empresa.Obter(ctx) // emp.Onboarding.Estado: configurando → perguntas → pronto
if emp.Onboarding.Estado == "perguntas" {
	pendentes, _ := c.Empresa.Onboarding(ctx) // o que o cadastro fiscal não respondeu sozinho
	fmt.Println(pendentes)
	_, err = c.Empresa.Fiscal(ctx, emissaoja.CamposFiscais{AliquotaSimples: 6}) // só com o valor que a empresa informou
}
```

A senha é conferida no portal e nunca volta em resposta. O portal bloqueia a conta da empresa depois de algumas senhas erradas: a API limita a 3 tentativas erradas por CNPJ em 24 h.

## Tomadores e serviços

```go
pf, err := c.Tomadores.Criar(ctx, "52998224725", "Maria da Silva") // CPF precisa do nome
pagina, err := c.Tomadores.Listar(ctx, &emissaoja.FiltroTomadores{Busca: "alfa", Ordem: "recentes"})
achados, err := c.Tomadores.Buscar(ctx, "12.345.678")
_, err = c.Tomadores.Atualizar(ctx, emissaoja.TomadorPJ, tomadorID, emissaoja.Campos{"email": "financeiro@cliente.com.br"})

catalogo, err := c.Servicos.Catalogo(ctx, "consultoria", 10) // código nacional (cTribNac) por texto
s, err := c.Servicos.Criar(ctx, emissaoja.CamposServico{
	Descricao: "Consultoria",
	CTribNac:  catalogo.Itens[0].CTribNac,
	TribISSQN: "tributavel",
	RetemISS:  false,
})
servicos, err := c.Servicos.Listar(ctx, nil) // emitir sem ServicoID usa o serviço principal
```

`TribISSQN` aceita `tributavel` e `nao_incidencia`; isenção, imunidade e exportação ficam fora do emissaoja. Edições parciais (`Atualizar`) recebem `emissaoja.Campos` com os nomes da API, e `nil` apaga um campo de texto. Tomador não se apaga, porque as notas apontam para ele.

## Notas

```go
lista, err := c.Notas.Listar(ctx, &emissaoja.FiltroNotas{Periodo: "mes_atual", Situacao: "emitidas"})
// lista.ProximoCursor vai em FiltroNotas.Cursor para buscar a próxima página

op, err := c.Notas.Cancelar(ctx, emissaoja.OrigemLocal, notaID, emissaoja.MotivoErroNaEmissao, "Valor digitado errado na nota")
fim, err := c.Operacoes.Aguardar(ctx, op.Operacao.ID, nil) // processando → concluida | falhou
```

Nota não se altera: para corrigir, cancele e emita outra. Nota autorizada e depois cancelada conta no plano; recusada não conta. `CamposNota.Valor` é `float64` em reais, e a API aceita até 2 casas decimais.

## Idempotência e nota em dobro

- Use uma referência por nota que você quer emitir (o id do seu pedido serve; 8 a 64 caracteres: letras, números, `_` e `-`). Repetir a mesma devolve o mesmo pedido com `Repetido == true`. Referência fora do formato é recusada antes de chamar a API (`chave_idempotencia_invalida`).
- `503 fila_indisponivel`, falha de `rede` ou `tempo_esgotado`: repita **com a mesma referência**.
- `409 possivel_duplicada`: já existe nota igual (tomador, valor e competência) no mesmo dia. Se for mesmo outra nota, repita com `PermitirRepetidaDe` = o `pedido_id` de `nota_igual_recente`.
- `409 emissao_repetida_recente`: a mesma nota com outra referência há menos de 2 minutos. Se for intencional, repita com `ConfirmoRepeticao: true`.
- A biblioteca não repete chamadas sozinha: emitir tem efeito fiscal, e quem decide repetir é você.

```go
r, err := c.Notas.Emitir(ctx, referencia, campos)
var e *emissaoja.Erro
if errors.As(err, &e) && e.Code == "possivel_duplicada" {
	igual, _ := e.Extras["nota_igual_recente"].(map[string]any)
	campos.PermitirRepetidaDe, _ = igual["pedido_id"].(string) // é mesmo outra nota
	r, err = c.Notas.Emitir(ctx, referencia, campos)
}
```

## Erros

Todo erro dos métodos é `*emissaoja.Erro`, com `Status` (HTTP; `0` quando nasce no cliente), `Code`, `Message`, `Extras` (os outros campos do corpo) e `RequestID` (cabeçalho `X-Request-Id`, para o suporte). Decida pelo `Code`, nunca pela mensagem:

```go
_, err := c.Notas.Emitir(ctx, "pedido-0002", campos)
var e *emissaoja.Erro
switch {
case err == nil:
	// pedido criado
case errors.As(err, &e) && e.Code == "nota_bloqueada":
	for _, b := range e.Bloqueios() { // o que falta no cadastro
		fmt.Println(b.Code, b.Acao, b.Mensagem)
	}
case emissaoja.CodigoDoErro(err) == "limite_notas":
	fmt.Println("limite do plano no mês")
default:
	log.Print(err) // emissaoja: <code> (HTTP <status>): <mensagem> [request_id ...]
}
```

| HTTP | `Code` | O que fazer |
| --- | --- | --- |
| 400 | `validation_error`, `valor_invalido`, `tomador_obrigatorio`, `cnpj_invalido` | Corrigir o campo (`Extras["details"]` diz qual) |
| 401 | `chave_invalida` | Conferir a chave (ou o ambiente: `ej_test_` só no sandbox) |
| 402 | `limite_notas` | Limite do plano no mês; planos em [emissaoja.com.br/#precos](https://emissaoja.com.br/#precos) |
| 404 | `tomador_nao_encontrado`, `servico_nao_encontrado`, `nota_nao_encontrada`, `pedido_nao_encontrado` | Conferir o id |
| 409 | `sem_empresa`, `aceite_pendente` | Conectar a empresa; aceitar os termos no app |
| 409 | `possivel_duplicada`, `emissao_repetida_recente`, `incerta_pendente` | Ver a seção acima; em `incerta_pendente`, não emita de novo e use `Emissoes.Conferir` |
| 409 | `ultimo_servico_ativo` | Cadastrar outro serviço antes de desativar |
| 422 | `nota_bloqueada` | Resolver `e.Bloqueios()` (cada um com `Code`, `Mensagem` e `Acao`) |
| 429 | `rate_limited` | Esperar `Extras["retry_after"]` segundos (cabeçalho `Retry-After`) |
| 503 | `motor_indisponivel`, `fila_indisponivel`, `receita_indisponivel` | Tentar de novo (na emissão, com a mesma referência) |
| 0 | `rede`, `tempo_esgotado`, `cancelado` | Falha de rede, prazo vencido ou contexto cancelado no seu lado |
| 0 | `chave_invalida`, `chave_idempotencia_invalida` | Sem chave (`ComChave` ou `EMISSAOJA_KEY`); referência fora do formato |

Limites: 60 chamadas por minuto por chave, 10 emissões por minuto, 120 por minuto em buscas e catálogo e 30 consultas de CNPJ por minuto.

## Referência

| Recurso | Métodos |
| --- | --- |
| `c.Empresa` | `Obter`, `Conectar`, `Credencial`, `AguardarCredencial`, `Onboarding`, `Fiscal`, `RelerFisco` |
| `c.Tomadores` | `Listar`, `Buscar`, `Criar`, `Obter`, `Atualizar` |
| `c.Servicos` | `Listar`, `Criar`, `Atualizar`, `Desativar`, `Catalogo` |
| `c.Notas` | `Emitir`, `Aguardar`, `Listar`, `PDF`, `XML`, `Cancelar` |
| `c.Emissoes` | `Obter`, `Listar`, `Conferir` |
| `c.Operacoes` | `Obter`, `Aguardar` |
| `c.CNPJ` | `Consultar` (dados cadastrais da Receita Federal) |
| `c` | `Status` (sem chave), `BaseURL` |

Opções de `NovoCliente`: `ComChave`, `ComBaseURL`, `ComTimeout` (padrão 30 s) e `ComHTTPClient`. Tipos, campos e exemplos: [pkg.go.dev/emissaoja.com.br/go](https://pkg.go.dev/emissaoja.com.br/go). Referência completa da API: [emissaoja.com.br/docs](https://emissaoja.com.br/docs/) e [OpenAPI 3.1](https://emissaoja.com.br/v1/openapi.json).

## Perguntas frequentes

**Precisa de certificado digital?** Não. O emissaoja entra no Emissor Nacional com o CNPJ e a senha do portal da própria empresa, o acesso que o portal oferece a MEI e empresas do Simples Nacional.

**Funciona para MEI?** Sim. O público é MEI, ME e EPP optantes pelo Simples Nacional, em um município que emite pelo Emissor Nacional.

**Tem ambiente de testes?** Sim, o sandbox em `https://sandbox.emissaoja.com.br` (`emissaoja.BaseSandbox`), com o Emissor Nacional simulado e empresas de exemplo.

**Quanto custa?** O plano Grátis emite 1 nota por mês; os planos pagos estão em [emissaoja.com.br/#precos](https://emissaoja.com.br/#precos). O sandbox é grátis. A nota conta quando o Emissor Nacional aceita; recusada não conta.

**Tem webhook?** Ainda não. Acompanhe o pedido com `c.Notas.Aguardar` ou `c.Emissoes.Obter`, que trazem `proxima_consulta_ms`.

## Outras formas de integrar

- Node.js/TypeScript: [`npm install emissaoja`](https://github.com/Easymovel/emissaoja-js) · Python: [`pip install emissaoja`](https://github.com/Easymovel/emissaoja-py) · PHP/Laravel: [`composer require emissaoja/emissaoja`](https://github.com/Easymovel/emissaoja-php) · .NET: [`dotnet add package Emissaoja`](https://github.com/Easymovel/emissaoja-dotnet) · Ruby: [`gem install emissaoja`](https://github.com/Easymovel/emissaoja-ruby)
- n8n: [`n8n-nodes-emissaoja`](https://github.com/Easymovel/n8n-nodes-emissaoja)
- Assistentes de IA (Claude, ChatGPT, Cursor, VS Code): servidor MCP em `https://emissaoja.com.br/mcp`
- Documentação: [emissaoja.com.br/docs](https://emissaoja.com.br/docs/) · [docs.md](https://emissaoja.com.br/docs.md) · [OpenAPI](https://emissaoja.com.br/v1/openapi.json) · para IAs: [llms.txt](https://emissaoja.com.br/llms.txt)

## In English

Official Go client for the [emissaoja API](https://emissaoja.com.br/docs/): issue Brazilian **NFS-e** (electronic service invoices) through the national **Emissor Nacional** without a digital certificate, for MEI and Simples Nacional companies. Standard library only, typed responses, `context.Context` everywhere, idempotent issuing, PDF and XML download, and a free sandbox with a simulated tax authority (`emissaoja.BaseSandbox`). Install with `go get emissaoja.com.br/go`; the API and the docs are in Portuguese.

---

Suporte: [suporte@emissaoja.com.br](mailto:suporte@emissaoja.com.br) · Status da API: [emissaoja.com.br/v1/status](https://emissaoja.com.br/v1/status). Licença [MIT](LICENSE), © 2026 BINOBO LTDA.

O emissaoja é um serviço privado e não tem vínculo com a Receita Federal nem com o Emissor Nacional.
