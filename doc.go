// Package emissaoja é o cliente oficial em Go da API do emissaoja: emita NFS-e (Nota Fiscal de
// Serviço eletrônica) no Emissor Nacional com o CNPJ e a senha do portal, sem certificado digital,
// para MEI e empresas do Simples Nacional.
//
// A API é a mesma que o app do emissaoja usa: conectar a empresa ao Emissor Nacional, cadastrar
// tomadores e serviços, emitir com idempotência, acompanhar o pedido, baixar o PDF (DANFSe) e o XML,
// cancelar e consultar CNPJ. O pacote usa só a biblioteca padrão. Site, cadastro e planos em
// https://emissaoja.com.br; referência completa da API em https://emissaoja.com.br/docs/.
//
// O caminho de import termina em "go", mas o pacote se chama emissaoja. Importe com o nome explícito:
//
//	import emissaoja "emissaoja.com.br/go"
//
// # Chave de API
//
// Entre no app com a conta Google (https://emissaoja.com.br/app), aceite os termos e crie a chave em
// Conta → API. Ela aparece uma vez e começa com ej_live_. A chave é secreta: use-a só no servidor,
// nunca em navegador ou em app distribuído. Sem [ComChave], o cliente lê a variável de ambiente
// EMISSAOJA_KEY na primeira chamada que precisa dela.
//
// # Fluxo mínimo
//
// Conecte a empresa uma vez com o CNPJ e a senha do Emissor Nacional. Depois, cadastre o tomador,
// emita com uma referência sua e acompanhe o pedido até o fim:
//
//	ctx := context.Background()
//	c := emissaoja.NovoCliente() // chave em EMISSAOJA_KEY
//
//	// Uma vez por empresa.
//	if _, err := c.Empresa.Conectar(ctx, emissaoja.CamposConexao{CNPJ: "11222333000181", Senha: "senha-do-portal"}); err != nil {
//		return err
//	}
//	if _, err := c.Empresa.AguardarCredencial(ctx, nil); err != nil {
//		return err
//	}
//
//	t, err := c.Tomadores.Criar(ctx, "12345678000195", "") // CNPJ: nome e endereço vêm da Receita
//	if err != nil {
//		return err
//	}
//	r, err := c.Notas.Emitir(ctx, "pedido-0001", emissaoja.CamposNota{
//		TomadorTipo: t.Tomador.Tipo,
//		TomadorID:   t.Tomador.ID,
//		Valor:       1500,
//		Descricao:   "Consultoria de outubro",
//	})
//	if err != nil {
//		return err
//	}
//	fim, err := c.Notas.Aguardar(ctx, r.Pedido.ID, nil) // autorizada, recusada ou incerta
//	if err != nil {
//		return err
//	}
//	if fim.Pedido.Estado == emissaoja.EstadoAutorizada {
//		pdf, err := c.Notas.PDF(ctx, fim.Pedido.Nota.Origem, fim.Pedido.Nota.ID)
//		// ...
//	}
//
// # Sandbox
//
// O [BaseSandbox] (https://sandbox.emissaoja.com.br) é o mesmo emissaoja com um Emissor Nacional
// simulado: nada vai ao fisco e nada é cobrado. Crie a conta com o Google, gere uma chave ej_test_ e
// conecte uma empresa de exemplo, como 11111111000191 (MEI pronto para emitir; qualquer senha, exceto
// "errada"). Tomadores de exemplo: CNPJ 12345678000195 e CPF 52998224725. Os valores 13.13, 77.77, 66.66
// e 99.99 simulam nota incerta, incerta que não saiu, recusada e bloqueada na prévia (nota_bloqueada).
//
//	c := emissaoja.NovoCliente(
//		emissaoja.ComBaseURL(emissaoja.BaseSandbox),
//		emissaoja.ComChave(os.Getenv("EMISSAOJA_TEST_KEY")),
//	)
//
// # Erros
//
// Todo erro dos métodos é *[Erro], com Status (HTTP; 0 quando nasce no cliente), Code (a lista fechada
// da API, como "nota_bloqueada" ou "limite_notas"), Message, Extras e RequestID. Decida pelo Code,
// nunca pela mensagem:
//
//	var e *emissaoja.Erro
//	if errors.As(err, &e) && e.Code == "nota_bloqueada" {
//		for _, b := range e.Bloqueios() {
//			fmt.Println(b.Code, b.Acao, b.Mensagem)
//		}
//	}
//
// Erros que nascem no cliente: "rede", "tempo_esgotado" (prazo da chamada, do contexto ou de um
// Aguardar), "cancelado" (contexto cancelado), "chave_invalida" (sem chave) e
// "chave_idempotencia_invalida" (referência fora do formato, recusada antes de chamar a API).
// [CodigoDoErro] devolve o Code de qualquer erro.
//
// # Idempotência
//
// [RecursoNotas.Emitir] exige uma referência sua por nota (8 a 64 caracteres: letras, números, _ e -),
// enviada no cabeçalho Idempotency-Key. Repetir a referência devolve o mesmo pedido, com Repetido igual
// a true, e nunca emite duas vezes. Este cliente não repete chamadas sozinho, porque emitir tem efeito
// fiscal: em "fila_indisponivel", "rede" ou "tempo_esgotado", chame Emitir de novo com a MESMA
// referência.
//
// # Acompanhamento
//
// Ainda não há webhooks. [RecursoNotas.Aguardar] consulta o pedido até autorizada, recusada ou incerta,
// no ritmo do proxima_consulta_ms que a API devolve, e para quando o contexto é cancelado. Todos os
// métodos recebem um context.Context primeiro, e o [Cliente] é seguro para uso concorrente.
package emissaoja
