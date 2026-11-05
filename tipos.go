package emissaoja

// TipoTomador diz se o tomador é pessoa jurídica (CNPJ) ou pessoa física (CPF).
type TipoTomador string

// Tipos de tomador.
const (
	// TomadorPJ é o tomador pessoa jurídica, com CNPJ.
	TomadorPJ TipoTomador = "pj"
	// TomadorPF é o tomador pessoa física, com CPF.
	TomadorPF TipoTomador = "pf"
)

// OrigemNota diz de onde veio a nota. Com o id, forma o par que baixa o PDF e o XML e que cancela a
// nota.
type OrigemNota string

// Origens de nota.
const (
	// OrigemLocal é a nota emitida pelo emissaoja.
	OrigemLocal OrigemNota = "local"
	// OrigemImportada é a nota trazida do Emissor Nacional, como as dos 90 dias antes da conexão.
	OrigemImportada OrigemNota = "importada"
)

// EstadoPedido é o estado de um pedido de emissão. Ele passa por na_fila, emitindo e confirmando e
// termina em autorizada, recusada ou incerta.
type EstadoPedido string

// Estados do pedido de emissão.
const (
	// EstadoNaFila é o pedido aceito, esperando a vez.
	EstadoNaFila EstadoPedido = "na_fila"
	// EstadoEmitindo é o pedido sendo enviado ao Emissor Nacional.
	EstadoEmitindo EstadoPedido = "emitindo"
	// EstadoConfirmando é o envio feito, com o resultado sendo conferido no portal.
	EstadoConfirmando EstadoPedido = "confirmando"
	// EstadoAutorizada é a nota emitida: o pedido traz Numero, ChaveAcesso e Nota.
	EstadoAutorizada EstadoPedido = "autorizada"
	// EstadoRecusada é a nota recusada pelo Emissor Nacional: veja MotivoCode e Motivo.
	EstadoRecusada EstadoPedido = "recusada"
	// EstadoIncerta é o pedido sem resposta clara do portal. Não emita de novo: peça a conferência com
	// RecursoEmissoes.Conferir.
	EstadoIncerta EstadoPedido = "incerta"
)

// Terminal informa se o estado é final: autorizada, recusada ou incerta.
func (e EstadoPedido) Terminal() bool {
	return e == EstadoAutorizada || e == EstadoRecusada || e == EstadoIncerta
}

// MotivoCancelamento é o motivo informado em [RecursoNotas.Cancelar].
type MotivoCancelamento string

// Motivos de cancelamento.
const (
	// MotivoErroNaEmissao ("1") é a nota emitida com erro.
	MotivoErroNaEmissao MotivoCancelamento = "1"
	// MotivoServicoNaoPrestado ("2") é o serviço que não foi prestado.
	MotivoServicoNaoPrestado MotivoCancelamento = "2"
)

// Campos é um corpo livre para as edições parciais ([RecursoTomadores.Atualizar] e
// [RecursoServicos.Atualizar]): mande só as chaves que mudam, com os nomes da API (snake_case).
type Campos map[string]any

// Ptr devolve um ponteiro para v. Serve para os campos opcionais em que o zero tem significado, como
// CamposNota.RetemISS: emissaoja.Ptr(false).
func Ptr[T any](v T) *T {
	return &v
}

// StatusAPI é a resposta de [Cliente.Status].
type StatusAPI struct {
	// OK é true quando a API está no ar.
	OK bool `json:"ok"`
	// Versao é a versão da API.
	Versao string `json:"versao"`
	// Motor é "ok", "degradado" ou "simulado" (sandbox).
	Motor string `json:"motor"`
	// Ambiente é "producao" ou "sandbox".
	Ambiente string `json:"ambiente"`
	// Tempo é a hora do servidor (ISO 8601).
	Tempo string `json:"tempo"`
}

// RespostaEmpresa é a resposta de [RecursoEmpresa.Obter].
type RespostaEmpresa struct {
	// Usuario é a conta dona da chave.
	Usuario Usuario `json:"usuario"`
	// Empresa é nil enquanto a empresa não foi conectada.
	Empresa *DadosEmpresa `json:"empresa"`
	// Credencial é o estado da senha do Emissor Nacional.
	Credencial Credencial `json:"credencial"`
	// Onboarding resume a configuração; a empresa emite quando Onboarding.Estado é "pronto".
	Onboarding EstadoOnboarding `json:"onboarding"`
	// Importacao acompanha a leitura das notas anteriores no portal; nil quando não há.
	Importacao map[string]any `json:"importacao"`
	// Avisos da conta, cada um com code.
	Avisos []map[string]any `json:"avisos"`
	// Aceite diz se há termos para aceitar no app.
	Aceite Aceite `json:"aceite"`
	// Tempo é a hora do servidor (ISO 8601).
	Tempo string `json:"tempo"`
}

// Usuario é a conta Google dona da chave.
type Usuario struct {
	// Email da conta.
	Email string `json:"email"`
}

// DadosEmpresa é a empresa conectada à conta.
type DadosEmpresa struct {
	// CNPJ só com números.
	CNPJ string `json:"cnpj"`
	// CNPJFormatado é o CNPJ com pontuação.
	CNPJFormatado string `json:"cnpj_formatado"`
	// Nome é a razão social.
	Nome string `json:"nome"`
	// PorteSimples é "mei" ou "me_epp"; vazio enquanto não se sabe.
	PorteSimples string `json:"porte_simples"`
	// Vinculo é o estado do vínculo da empresa com o emissaoja.
	Vinculo string `json:"vinculo"`
}

// EstadoOnboarding resume a configuração da empresa.
type EstadoOnboarding struct {
	// Estado é acesso_restrito, aceite, verificando, bloqueado, credencial, credencial_congelada,
	// configurando, perguntas ou pronto. Em "perguntas", leia RecursoEmpresa.Onboarding e responda com
	// RecursoEmpresa.Fiscal.
	Estado string `json:"estado"`
}

// Aceite diz se há termos para aceitar no app.
type Aceite struct {
	// Pendente é true quando falta aceitar os termos em https://emissaoja.com.br/app.
	Pendente bool `json:"pendente"`
}

// Credencial é o estado da senha do Emissor Nacional na conta.
type Credencial struct {
	// Estado é "ausente", "verificando", "ativa" ou "congelada" (o portal recusou a senha; conecte de
	// novo com RecursoEmpresa.Conectar).
	Estado string `json:"estado"`
	// CongeladaEm é quando a credencial congelou (ISO 8601).
	CongeladaEm string `json:"congelada_em"`
	// TentativaEmVoo é o id da verificação de senha em andamento, se houver.
	TentativaEmVoo string `json:"tentativa_em_voo"`
}

// Tentativa é uma verificação da senha no Emissor Nacional.
type Tentativa struct {
	// ID da tentativa.
	ID string `json:"id"`
	// Estado é verificando, aceita, senha_recusada, portal_indisponivel, bloqueada, interrompida ou erro.
	Estado string `json:"estado"`
	// ProximoPasso diz o que fazer: aguardar, configurando, pronto, corrigir_senha, tentar_mais_tarde,
	// redefinir_no_portal ou suporte.
	ProximoPasso string `json:"proximo_passo"`
	// Code detalha a falha, quando há.
	Code string `json:"code"`
}

// RespostaConexao é a resposta de [RecursoEmpresa.Conectar].
type RespostaConexao struct {
	// Tentativa é a verificação iniciada; acompanhe com RecursoEmpresa.AguardarCredencial.
	Tentativa Tentativa `json:"tentativa"`
}

// RespostaCredencial é a resposta de [RecursoEmpresa.Credencial] e [RecursoEmpresa.AguardarCredencial].
type RespostaCredencial struct {
	// Credencial é o estado da senha na conta.
	Credencial Credencial `json:"credencial"`
	// Onboarding resume a configuração da empresa.
	Onboarding EstadoOnboarding `json:"onboarding"`
	// Tentativa é a verificação em andamento ou a última; nil quando nunca houve.
	Tentativa *Tentativa `json:"tentativa"`
}

// Tomador é o cliente que recebe a nota. Endereço e contato vêm na ficha ([RecursoTomadores.Obter] e
// [RecursoTomadores.Atualizar]); NotasEmitidas e UltimaNotaEm, na listagem.
type Tomador struct {
	// Tipo é TomadorPJ ou TomadorPF.
	Tipo TipoTomador `json:"tipo"`
	// ID do tomador, usado em CamposNota.TomadorID.
	ID string `json:"id"`
	// Nome ou razão social.
	Nome string `json:"nome"`
	// Documento é o CNPJ ou o CPF só com números.
	Documento string `json:"documento"`
	// DocumentoFormatado é o documento com pontuação.
	DocumentoFormatado string `json:"documento_formatado"`
	// Cidade do endereço.
	Cidade string `json:"cidade"`
	// UF do endereço.
	UF string `json:"uf"`
	// NotasEmitidas para este tomador.
	NotasEmitidas int `json:"notas_emitidas"`
	// UltimaNotaEm é a data da última nota para este tomador (ISO 8601).
	UltimaNotaEm string `json:"ultima_nota_em"`
	// CEP do endereço.
	CEP string `json:"cep"`
	// Endereco é o logradouro.
	Endereco string `json:"endereco"`
	// Numero do endereço.
	Numero string `json:"numero"`
	// Complemento do endereço.
	Complemento string `json:"complemento"`
	// Bairro do endereço.
	Bairro string `json:"bairro"`
	// CodigoMunicipioIBGE é o código IBGE da cidade.
	CodigoMunicipioIBGE string `json:"codigo_municipio_ibge"`
	// Telefone de contato.
	Telefone string `json:"telefone"`
	// Email de contato.
	Email string `json:"email"`
	// InscricaoMunicipal do tomador, quando há.
	InscricaoMunicipal string `json:"inscricao_municipal"`
}

// ListaTomadores é a resposta de [RecursoTomadores.Listar] e [RecursoTomadores.Buscar]; a busca só
// preenche Tomadores.
type ListaTomadores struct {
	// Tomadores da página.
	Tomadores []Tomador `json:"tomadores"`
	// Total de tomadores no filtro.
	Total int `json:"total"`
	// Pagina atual.
	Pagina int `json:"pagina"`
	// Limite por página.
	Limite int `json:"limite"`
}

// RespostaTomador é a resposta de [RecursoTomadores.Criar], [RecursoTomadores.Obter] e
// [RecursoTomadores.Atualizar].
type RespostaTomador struct {
	// Tomador cadastrado.
	Tomador Tomador `json:"tomador"`
	// JaExistia é true quando Criar encontrou o tomador já cadastrado.
	JaExistia bool `json:"ja_existia"`
}

// Servico é um serviço que a empresa presta, com o código de tributação que vai na nota.
type Servico struct {
	// ID do serviço, usado em CamposNota.ServicoID.
	ID string `json:"id"`
	// Descricao é o nome do serviço.
	Descricao string `json:"descricao"`
	// DescricaoDaNota é a descrição que vai na nota, quando definida.
	DescricaoDaNota string `json:"descricao_da_nota"`
	// Principal marca o serviço usado quando a nota não informa ServicoID.
	Principal bool `json:"principal"`
	// Ativo é false nos serviços desativados.
	Ativo bool `json:"ativo"`
	// CTribNac é o código de tributação nacional.
	CTribNac string `json:"c_trib_nac"`
	// CTribMun é o código do serviço na prefeitura, quando há.
	CTribMun string `json:"c_trib_mun"`
	// ItemLC116 é o item da lista da Lei Complementar 116.
	ItemLC116 string `json:"item_lc116"`
	// NBS é o código NBS, quando há.
	NBS string `json:"nbs"`
	// TribISSQN é "tributavel" ou "nao_incidencia".
	TribISSQN string `json:"trib_issqn"`
	// TpOper é o tipo de operação (1 a 5); nil quando não definido.
	TpOper *int `json:"tp_oper"`
	// RetemISS diz se o tomador retém o ISS.
	RetemISS bool `json:"retem_iss"`
	// AliquotaISS é a alíquota do ISS em %; nil quando não definida.
	AliquotaISS *float64 `json:"aliquota_iss"`
}

// ListaServicos é a resposta de [RecursoServicos.Listar].
type ListaServicos struct {
	// Servicos cadastrados.
	Servicos []Servico `json:"servicos"`
}

// RespostaServico é a resposta de [RecursoServicos.Criar], [RecursoServicos.Atualizar] e
// [RecursoServicos.Desativar].
type RespostaServico struct {
	// Servico como ficou.
	Servico Servico `json:"servico"`
	// VinculosAtualizados conta os vínculos que a edição atualizou (só em Atualizar).
	VinculosAtualizados int `json:"vinculos_atualizados"`
}

// ItemCatalogo é um código do catálogo nacional de serviços.
type ItemCatalogo struct {
	// CTribNac é o código de tributação nacional (6 dígitos), usado em CamposServico.CTribNac.
	CTribNac string `json:"c_trib_nac"`
	// Descricao do código.
	Descricao string `json:"descricao"`
	// ItemLC116 é o item da lista da Lei Complementar 116.
	ItemLC116 string `json:"item_lc116"`
	// NBSSugeridos são os códigos NBS ligados ao item.
	NBSSugeridos []NBSSugerido `json:"nbs_sugeridos"`
}

// NBSSugerido é um código NBS sugerido para um item do catálogo.
type NBSSugerido struct {
	// NBS é o código.
	NBS string `json:"nbs"`
	// Descricao do código NBS.
	Descricao string `json:"descricao"`
}

// ListaCatalogo é a resposta de [RecursoServicos.Catalogo].
type ListaCatalogo struct {
	// Itens encontrados.
	Itens []ItemCatalogo `json:"itens"`
}

// Pedido é um pedido de emissão, do envio até autorizada, recusada ou incerta.
type Pedido struct {
	// ID do pedido.
	ID string `json:"id"`
	// Estado do pedido; Estado.Terminal() diz se ele acabou.
	Estado EstadoPedido `json:"estado"`
	// MotivoCode é o código do motivo, em recusada e incerta.
	MotivoCode string `json:"motivo_code"`
	// Motivo é o texto da recusa do fisco, quando há.
	Motivo string `json:"motivo"`
	// Numero é o número da NFS-e, em autorizada.
	Numero string `json:"numero"`
	// ChaveAcesso é a chave de acesso da NFS-e, em autorizada.
	ChaveAcesso string `json:"chave_acesso"`
	// ConsultaPublicaURL abre a nota na consulta pública do Emissor Nacional.
	ConsultaPublicaURL string `json:"consulta_publica_url"`
	// Nota é o par origem e id que baixa o PDF e o XML; nil antes de autorizada.
	Nota *RefNota `json:"nota"`
	// Acoes são as ações possíveis agora, como "baixar_pdf" ou "conferir".
	Acoes []string `json:"acoes"`
	// ProximaConsultaMs diz em quantos milissegundos consultar de novo; nil quando o pedido terminou.
	ProximaConsultaMs *int `json:"proxima_consulta_ms"`
}

// RefNota identifica uma nota pela origem e pelo id.
type RefNota struct {
	// Origem é OrigemLocal ou OrigemImportada.
	Origem OrigemNota `json:"origem"`
	// ID da nota.
	ID string `json:"id"`
}

// Previa resume como a nota sai. Vem na resposta de [RecursoNotas.Emitir].
type Previa struct {
	// Resumo da nota (tomador, serviço e valores), em formato livre.
	Resumo map[string]any `json:"resumo"`
	// Avisos que não impedem a emissão, como retenções.
	Avisos []Aviso `json:"avisos"`
}

// Aviso é um alerta da prévia que não impede a emissão.
type Aviso struct {
	// Code do aviso.
	Code string `json:"code"`
	// Mensagem em português.
	Mensagem string `json:"mensagem"`
	// Acao sugerida.
	Acao string `json:"acao"`
	// Detalhe traz valores do aviso, como irrf e csrf; nil quando não há.
	Detalhe map[string]any `json:"detalhe"`
}

// RespostaEmissao é a resposta de [RecursoNotas.Emitir].
type RespostaEmissao struct {
	// Pedido criado, ou o original quando Repetido.
	Pedido Pedido `json:"pedido"`
	// Previa da nota, com os avisos.
	Previa *Previa `json:"previa,omitempty"`
	// Repetido é true quando a referência já tinha sido usada: a API devolveu o pedido original e não
	// emitiu de novo.
	Repetido bool `json:"repetido,omitempty"`
}

// RespostaPedido é a resposta de [RecursoEmissoes.Obter], [RecursoEmissoes.Conferir] e
// [RecursoNotas.Aguardar].
type RespostaPedido struct {
	// Pedido no estado atual.
	Pedido Pedido `json:"pedido"`
}

// ListaPedidos é a resposta de [RecursoEmissoes.Listar].
type ListaPedidos struct {
	// Pedidos encontrados.
	Pedidos []Pedido `json:"pedidos"`
}

// Nota é uma NFS-e emitida pelo emissaoja ou importada do Emissor Nacional.
type Nota struct {
	// Origem é OrigemLocal ou OrigemImportada.
	Origem OrigemNota `json:"origem"`
	// ID da nota; com Origem, baixa o PDF e o XML e cancela a nota.
	ID string `json:"id"`
	// Numero da NFS-e.
	Numero string `json:"numero"`
	// Situacao é emitida, cancelada, cancelando, emitindo, conferindo ou nao_emitida.
	Situacao string `json:"situacao"`
	// EmitidaEm é a data de emissão (ISO 8601).
	EmitidaEm string `json:"emitida_em"`
	// Competencia é a data do serviço (AAAA-MM-DD); vazia nas importadas.
	Competencia string `json:"competencia"`
	// Tomador da nota.
	Tomador ResumoTomador `json:"tomador"`
	// Descricao do serviço na nota.
	Descricao string `json:"descricao"`
	// Valor da nota em reais.
	Valor float64 `json:"valor"`
	// ChaveAcesso é a chave de acesso da NFS-e.
	ChaveAcesso string `json:"chave_acesso"`
	// PedidoID é o pedido que emitiu a nota; vazio nas importadas.
	PedidoID string `json:"pedido_id"`
}

// ResumoTomador é o tomador como aparece na lista de notas.
type ResumoTomador struct {
	// Tipo é TomadorPJ ou TomadorPF.
	Tipo TipoTomador `json:"tipo"`
	// Nome ou razão social.
	Nome string `json:"nome"`
	// Documento só com números.
	Documento string `json:"documento"`
	// DocumentoFormatado é o documento com pontuação.
	DocumentoFormatado string `json:"documento_formatado"`
}

// ListaNotas é a resposta de [RecursoNotas.Listar], paginada por cursor.
type ListaNotas struct {
	// Notas da página.
	Notas []Nota `json:"notas"`
	// Total de notas no filtro.
	Total int `json:"total"`
	// JanelaCheia é true quando o período tem mais notas do que as 300 mais recentes que a consulta
	// alcança; estreite o período para ver as outras.
	JanelaCheia bool `json:"janela_cheia"`
	// ProximoCursor vai em FiltroNotas.Cursor para buscar a próxima página; vazio na última.
	ProximoCursor string `json:"proximo_cursor"`
}

// Operacao é uma operação em segundo plano, como o cancelamento de uma nota.
type Operacao struct {
	// ID da operação.
	ID string `json:"id"`
	// Tipo da operação, como "cancelamento".
	Tipo string `json:"tipo"`
	// Estado é "processando", "concluida" ou "falhou".
	Estado string `json:"estado"`
	// Alvo é a nota da operação, quando há.
	Alvo *RefNota `json:"alvo"`
	// Resultado da operação concluída, em formato livre.
	Resultado map[string]any `json:"resultado"`
	// Erro explica a operação que falhou.
	Erro *ErroOperacao `json:"erro"`
}

// ErroOperacao é o motivo de uma operação que falhou.
type ErroOperacao struct {
	// Code da falha, como "cancelamento_fora_do_prazo".
	Code string `json:"code"`
	// Message em português.
	Message string `json:"message"`
	// MotivoFisco é o texto do Emissor Nacional, quando há.
	MotivoFisco string `json:"motivo_fisco"`
}

// RespostaOperacao é a resposta de [RecursoNotas.Cancelar], [RecursoOperacoes.Obter] e
// [RecursoOperacoes.Aguardar].
type RespostaOperacao struct {
	// Operacao no estado atual.
	Operacao Operacao `json:"operacao"`
}
