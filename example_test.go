package emissaoja_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	emissaoja "emissaoja.com.br/go"
)

// O fluxo completo no sandbox, onde nada vai ao fisco: conectar a empresa de exemplo, cadastrar o
// tomador, emitir, acompanhar o pedido e baixar o PDF e o XML. Use uma chave ej_test_ do sandbox em
// EMISSAOJA_KEY.
func Example() {
	ctx := context.Background()
	c := emissaoja.NovoCliente(emissaoja.ComBaseURL(emissaoja.BaseSandbox)) // chave em EMISSAOJA_KEY

	// 1. Conectar a empresa, uma vez. No sandbox, 11111111000191 é um MEI pronto para emitir.
	if _, err := c.Empresa.Conectar(ctx, emissaoja.CamposConexao{CNPJ: "11111111000191", Senha: "qualquer-senha"}); err != nil {
		log.Fatal(err)
	}
	cred, err := c.Empresa.AguardarCredencial(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("credencial:", cred.Credencial.Estado)

	// 2. Tomador pelo CNPJ: nome e endereço vêm da Receita Federal.
	t, err := c.Tomadores.Criar(ctx, "12345678000195", "")
	if err != nil {
		log.Fatal(err)
	}

	// 3. Emitir com uma referência sua: repetir a mesma referência nunca emite duas notas.
	r, err := c.Notas.Emitir(ctx, "pedido-2026-11-0001", emissaoja.CamposNota{
		TomadorTipo: t.Tomador.Tipo,
		TomadorID:   t.Tomador.ID,
		Valor:       1500,
		Descricao:   "Consultoria em marketing digital, novembro de 2026",
	})
	if err != nil {
		log.Fatal(err)
	}

	// 4. Acompanhar até autorizada, recusada ou incerta, no ritmo que a API indica.
	fim, err := c.Notas.Aguardar(ctx, r.Pedido.ID, nil)
	if err != nil {
		log.Fatal(err)
	}
	if fim.Pedido.Estado != emissaoja.EstadoAutorizada {
		log.Fatalf("pedido %s terminou em %s: %s", fim.Pedido.ID, fim.Pedido.Estado, fim.Pedido.Motivo)
	}
	fmt.Println("NFS-e", fim.Pedido.Numero, "autorizada; chave de acesso", fim.Pedido.ChaveAcesso)

	// 5. Baixar o PDF (DANFSe) e o XML.
	pdf, err := c.Notas.PDF(ctx, fim.Pedido.Nota.Origem, fim.Pedido.Nota.ID)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("nfse-"+fim.Pedido.Numero+".pdf", pdf, 0o600); err != nil {
		log.Fatal(err)
	}
	xml, err := c.Notas.XML(ctx, fim.Pedido.Nota.Origem, fim.Pedido.Nota.ID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(xml), "caracteres de XML")
}

// Em produção, a chave vem de EMISSAOJA_KEY. No sandbox, use uma chave ej_test_.
func ExampleNovoCliente() {
	producao := emissaoja.NovoCliente()

	sandbox := emissaoja.NovoCliente(
		emissaoja.ComBaseURL(emissaoja.BaseSandbox),
		emissaoja.ComChave(os.Getenv("EMISSAOJA_TEST_KEY")),
		emissaoja.ComTimeout(10*time.Second),
	)

	fmt.Println(producao.BaseURL())
	fmt.Println(sandbox.BaseURL())
}

// Status não precisa de chave e diz em que ambiente o cliente está.
func ExampleCliente_Status() {
	c := emissaoja.NovoCliente(emissaoja.ComBaseURL(emissaoja.BaseSandbox))
	st, err := c.Status(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(st.Ambiente, st.Motor, st.Versao)
}

// Tomador com CNPJ só precisa do documento; com CPF, também do nome completo.
func ExampleRecursoTomadores_Criar() {
	ctx := context.Background()
	c := emissaoja.NovoCliente()

	pj, err := c.Tomadores.Criar(ctx, "12345678000195", "")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(pj.Tomador.Nome, pj.Tomador.Cidade, pj.Tomador.UF, "já existia:", pj.JaExistia)

	pf, err := c.Tomadores.Criar(ctx, "52998224725", "Maria da Silva")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(pf.Tomador.ID, pf.Tomador.DocumentoFormatado)
}

// O código de tributação nacional vem do catálogo; com ele, o serviço é cadastrado.
func ExampleRecursoServicos_Criar() {
	ctx := context.Background()
	c := emissaoja.NovoCliente()

	catalogo, err := c.Servicos.Catalogo(ctx, "consultoria", 5)
	if err != nil {
		log.Fatal(err)
	}
	if len(catalogo.Itens) == 0 {
		log.Fatal("nenhum código encontrado")
	}
	s, err := c.Servicos.Criar(ctx, emissaoja.CamposServico{
		Descricao: "Consultoria em gestão",
		CTribNac:  catalogo.Itens[0].CTribNac,
		TribISSQN: "tributavel",
		RetemISS:  false,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(s.Servico.ID, s.Servico.CTribNac, s.Servico.ItemLC116)
}

// Decida pelo Code do erro. Nota igual a outra de hoje pede confirmação apontando a anterior; fila
// indisponível, rede e tempo esgotado pedem repetir com a mesma referência, que nunca emite duas vezes.
func ExampleRecursoNotas_Emitir() {
	ctx := context.Background()
	c := emissaoja.NovoCliente()

	referencia := "venda-2026-11-0042" // o id do seu pedido identifica a nota
	campos := emissaoja.CamposNota{
		TomadorTipo: emissaoja.TomadorPJ,
		TomadorID:   "id-do-tomador",
		Valor:       250.75,
		Descricao:   "Manutenção mensal do site",
	}

	r, err := c.Notas.Emitir(ctx, referencia, campos)
	var e *emissaoja.Erro
	if errors.As(err, &e) && e.Code == "possivel_duplicada" {
		// Já saiu hoje uma nota igual. Se for mesmo outra, confirme apontando a anterior.
		if igual, ok := e.Extras["nota_igual_recente"].(map[string]any); ok {
			campos.PermitirRepetidaDe, _ = igual["pedido_id"].(string)
			r, err = c.Notas.Emitir(ctx, referencia, campos)
		}
	}
	switch {
	case err == nil:
		fmt.Println("pedido", r.Pedido.ID, r.Pedido.Estado, "repetido:", r.Repetido)
	case errors.As(err, &e) && e.Code == "nota_bloqueada":
		for _, b := range e.Bloqueios() {
			fmt.Println("falta resolver:", b.Code, b.Acao, b.Mensagem)
		}
	case errors.As(err, &e) && e.Code == "limite_notas":
		fmt.Println("limite do plano no mês; planos em https://emissaoja.com.br/#precos")
	case errors.As(err, &e) && (e.Code == "fila_indisponivel" || e.Code == "rede" || e.Code == "tempo_esgotado"):
		fmt.Println("tente de novo mais tarde com a mesma referência:", referencia)
	default:
		log.Fatal(err)
	}
}

// Aguardar segue o pedido até o fim. Incerta quer dizer que o portal não respondeu com clareza: não
// emita de novo, peça a conferência.
func ExampleRecursoNotas_Aguardar() {
	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancelar()
	c := emissaoja.NovoCliente()

	fim, err := c.Notas.Aguardar(ctx, "id-do-pedido", &emissaoja.AguardarOpcoes{Timeout: 3 * time.Minute})
	if err != nil {
		log.Fatal(err) // "tempo_esgotado": consulte de novo mais tarde
	}
	switch fim.Pedido.Estado {
	case emissaoja.EstadoAutorizada:
		fmt.Println("autorizada:", fim.Pedido.Numero, fim.Pedido.ConsultaPublicaURL)
	case emissaoja.EstadoRecusada:
		fmt.Println("recusada:", fim.Pedido.MotivoCode, fim.Pedido.Motivo)
	case emissaoja.EstadoIncerta:
		if _, err := c.Emissoes.Conferir(ctx, fim.Pedido.ID); err != nil {
			log.Fatal(err)
		}
		fmt.Println("conferência pedida; acompanhe de novo com Aguardar")
	}
}

// Listar pagina por cursor: repita com o ProximoCursor até ele voltar vazio.
func ExampleRecursoNotas_Listar() {
	ctx := context.Background()
	c := emissaoja.NovoCliente()

	filtro := &emissaoja.FiltroNotas{Periodo: "mes_atual", Situacao: "emitidas", Limite: 50}
	for {
		pagina, err := c.Notas.Listar(ctx, filtro)
		if err != nil {
			log.Fatal(err)
		}
		for _, n := range pagina.Notas {
			fmt.Printf("%s %s %s R$ %.2f\n", n.Numero, n.EmitidaEm, n.Tomador.Nome, n.Valor)
		}
		if pagina.ProximoCursor == "" {
			break
		}
		filtro.Cursor = pagina.ProximoCursor
	}
}

// Cancelar devolve uma operação em segundo plano; Operacoes.Aguardar espera o resultado.
func ExampleRecursoNotas_Cancelar() {
	ctx := context.Background()
	c := emissaoja.NovoCliente()

	op, err := c.Notas.Cancelar(ctx, emissaoja.OrigemLocal, "id-da-nota", emissaoja.MotivoErroNaEmissao, "Valor digitado errado na nota")
	if err != nil {
		log.Fatal(err)
	}
	fim, err := c.Operacoes.Aguardar(ctx, op.Operacao.ID, nil)
	if err != nil {
		log.Fatal(err)
	}
	if fim.Operacao.Erro != nil {
		fmt.Println("não cancelou:", fim.Operacao.Erro.Code, fim.Operacao.Erro.Message)
		return
	}
	fmt.Println("operação", fim.Operacao.Estado)
}

// Bloqueios lista o que falta para a nota sair, com a ação que resolve cada item.
func ExampleErro_Bloqueios() {
	c := emissaoja.NovoCliente()
	_, err := c.Notas.Emitir(context.Background(), "pedido-0002", emissaoja.CamposNota{TomadorTipo: emissaoja.TomadorPJ, TomadorID: "id-do-tomador", Valor: 100})

	var e *emissaoja.Erro
	if errors.As(err, &e) && e.Code == "nota_bloqueada" {
		for _, b := range e.Bloqueios() {
			fmt.Printf("%s: %s (ação: %s)\n", b.Code, b.Mensagem, b.Acao)
		}
	}
}

// CodigoDoErro lê o Code sem errors.As e devolve "" quando não houve erro.
func ExampleCodigoDoErro() {
	c := emissaoja.NovoCliente()
	_, err := c.Empresa.Obter(context.Background())
	switch emissaoja.CodigoDoErro(err) {
	case "":
		fmt.Println("tudo certo")
	case "chave_invalida":
		fmt.Println("confira a chave em EMISSAOJA_KEY; a ej_test_ só vale no sandbox")
	case "aceite_pendente":
		fmt.Println("aceite os termos em https://emissaoja.com.br/app")
	case "sem_empresa":
		fmt.Println("conecte a empresa com c.Empresa.Conectar")
	case "rate_limited":
		fmt.Println("limite por minuto; espere e repita")
	default:
		log.Fatal(err)
	}
}
