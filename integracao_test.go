package emissaoja_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	emissaoja "emissaoja.com.br/go"
)

// TestSandbox roda o fluxo inteiro no sandbox (https://sandbox.emissaoja.com.br), com o Emissor Nacional
// simulado. Só roda com EMISSAOJA_TEST_KEY definida (uma chave ej_test_); sem ela, e no CI, é pulado:
//
//	EMISSAOJA_TEST_KEY=ej_test_... go test -run TestSandbox -v ./...
func TestSandbox(t *testing.T) {
	chave := os.Getenv("EMISSAOJA_TEST_KEY")
	if chave == "" {
		t.Skip("defina EMISSAOJA_TEST_KEY (chave ej_test_ do sandbox) para rodar o teste de integração")
	}
	ctx, cancelar := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancelar()
	c := emissaoja.NovoCliente(emissaoja.ComBaseURL(emissaoja.BaseSandbox), emissaoja.ComChave(chave))

	st, err := c.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Ambiente != "sandbox" {
		t.Fatalf("esperava o sandbox, veio o ambiente %q", st.Ambiente)
	}

	emp, err := c.Empresa.Obter(ctx)
	if err != nil {
		t.Fatalf("Empresa.Obter: %v", err)
	}
	if emp.Empresa == nil {
		if _, err := c.Empresa.Conectar(ctx, emissaoja.CamposConexao{CNPJ: "11111111000191", Senha: "senha-de-teste"}); err != nil {
			t.Fatalf("Empresa.Conectar: %v", err)
		}
		cred, err := c.Empresa.AguardarCredencial(ctx, nil)
		if err != nil {
			t.Fatalf("Empresa.AguardarCredencial: %v", err)
		}
		if p := cred.Tentativa; p != nil && p.ProximoPasso != "pronto" && p.ProximoPasso != "configurando" {
			t.Fatalf("a senha não foi aceita: %+v", *p)
		}
	}
	esperarEmpresaPronta(t, ctx, c)

	tom, err := c.Tomadores.Criar(ctx, "12345678000195", "")
	if err != nil {
		t.Fatalf("Tomadores.Criar: %v", err)
	}
	if tom.Tomador.Tipo != emissaoja.TomadorPJ || tom.Tomador.ID == "" {
		t.Fatalf("tomador = %+v", tom.Tomador)
	}

	referencia := "go-" + time.Now().UTC().Format("20060102-150405")
	campos := emissaoja.CamposNota{TomadorTipo: tom.Tomador.Tipo, TomadorID: tom.Tomador.ID, Valor: 150, Descricao: "Teste de integração do cliente Go"}
	r, err := emitirConfirmando(ctx, c, referencia, &campos)
	if emissaoja.CodigoDoErro(err) == "limite_notas" {
		t.Skipf("limite de notas do plano: %v", err)
	}
	if err != nil {
		t.Fatalf("Notas.Emitir: %v", err)
	}

	dobro, err := c.Notas.Emitir(ctx, referencia, campos)
	if err != nil {
		t.Fatalf("Notas.Emitir com a mesma referência: %v", err)
	}
	if !dobro.Repetido || dobro.Pedido.ID != r.Pedido.ID {
		t.Errorf("a mesma referência devia devolver o mesmo pedido: %+v", dobro.Pedido)
	}

	fim, err := c.Notas.Aguardar(ctx, r.Pedido.ID, nil)
	if err != nil {
		t.Fatalf("Notas.Aguardar: %v", err)
	}
	if fim.Pedido.Estado != emissaoja.EstadoAutorizada || fim.Pedido.Nota == nil {
		t.Fatalf("esperava autorizada, veio %+v", fim.Pedido)
	}

	pdf, err := c.Notas.PDF(ctx, fim.Pedido.Nota.Origem, fim.Pedido.Nota.ID)
	if err != nil {
		t.Fatalf("Notas.PDF: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF") {
		t.Errorf("o PDF não começa com %%PDF (%d bytes)", len(pdf))
	}
	xml, err := c.Notas.XML(ctx, fim.Pedido.Nota.Origem, fim.Pedido.Nota.ID)
	if err != nil {
		t.Fatalf("Notas.XML: %v", err)
	}
	if !strings.Contains(xml, "<") {
		t.Errorf("XML inesperado: %q", xml)
	}

	notas, err := c.Notas.Listar(ctx, &emissaoja.FiltroNotas{Periodo: "mes_atual", Limite: 50})
	if err != nil {
		t.Fatalf("Notas.Listar: %v", err)
	}
	t.Logf("%d notas no mês", notas.Total)

	op, err := c.Notas.Cancelar(ctx, fim.Pedido.Nota.Origem, fim.Pedido.Nota.ID, emissaoja.MotivoErroNaEmissao, "Teste de integração do cliente Go")
	if err != nil {
		t.Fatalf("Notas.Cancelar: %v", err)
	}
	opFim, err := c.Operacoes.Aguardar(ctx, op.Operacao.ID, nil)
	if err != nil {
		t.Fatalf("Operacoes.Aguardar: %v", err)
	}
	if opFim.Operacao.Estado != "concluida" && opFim.Operacao.Estado != "falhou" {
		t.Errorf("operação = %+v", opFim.Operacao)
	}
}

// esperarEmpresaPronta espera o onboarding da empresa chegar a "pronto", por até 2 minutos.
func esperarEmpresaPronta(t *testing.T, ctx context.Context, c *emissaoja.Cliente) {
	t.Helper()
	limite := time.Now().Add(2 * time.Minute)
	for {
		emp, err := c.Empresa.Obter(ctx)
		if err != nil {
			t.Fatalf("Empresa.Obter: %v", err)
		}
		if emp.Onboarding.Estado == "pronto" {
			return
		}
		if time.Now().After(limite) {
			t.Fatalf("a empresa não ficou pronta; onboarding em %q", emp.Onboarding.Estado)
		}
		time.Sleep(2 * time.Second)
	}
}

// emitirConfirmando emite e, se a API pedir confirmação de nota repetida, confirma uma vez. Os campos
// confirmados ficam em campos, para repetir a mesma emissão depois.
func emitirConfirmando(ctx context.Context, c *emissaoja.Cliente, referencia string, campos *emissaoja.CamposNota) (*emissaoja.RespostaEmissao, error) {
	r, err := c.Notas.Emitir(ctx, referencia, *campos)
	var e *emissaoja.Erro
	if !errors.As(err, &e) {
		return r, err
	}
	switch e.Code {
	case "possivel_duplicada":
		igual, _ := e.Extras["nota_igual_recente"].(map[string]any)
		campos.PermitirRepetidaDe, _ = igual["pedido_id"].(string)
	case "emissao_repetida_recente":
		campos.ConfirmoRepeticao = true
	default:
		return nil, err
	}
	return c.Notas.Emitir(ctx, referencia, *campos)
}
