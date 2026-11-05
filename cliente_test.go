package emissaoja

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const chaveTeste = "ej_test_chave_de_teste"

// chamada é uma requisição que o servidor de teste recebeu.
type chamada struct {
	metodo string
	uri    string
	cab    http.Header
	corpo  []byte
}

// gravador guarda as chamadas recebidas; o handler roda em outra goroutine.
type gravador struct {
	mu       sync.Mutex
	chamadas []chamada
}

func (g *gravador) registrar(ch chamada) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.chamadas = append(g.chamadas, ch)
}

func (g *gravador) lista() []chamada {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]chamada(nil), g.chamadas...)
}

func (g *gravador) ultima(t *testing.T) chamada {
	t.Helper()
	todas := g.lista()
	if len(todas) == 0 {
		t.Fatal("nenhuma chamada chegou ao servidor de teste")
	}
	return todas[len(todas)-1]
}

// novoServidor sobe um servidor de teste com o handler e devolve um cliente apontado para ele, com
// chaveTeste. As opções extras valem por cima das padrão.
func novoServidor(t *testing.T, h http.HandlerFunc, opcoes ...Opcao) (*Cliente, *gravador) {
	t.Helper()
	g := &gravador{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corpo, _ := io.ReadAll(r.Body)
		g.registrar(chamada{metodo: r.Method, uri: r.RequestURI, cab: r.Header.Clone(), corpo: corpo})
		r.Body = io.NopCloser(bytes.NewReader(corpo))
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	todas := append([]Opcao{ComChave(chaveTeste), ComBaseURL(srv.URL)}, opcoes...)
	return NovoCliente(todas...), g
}

// responderJSON escreve corpo como JSON, com o cabeçalho X-Request-Id.
func responderJSON(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-Id", "req-teste")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(corpo)
}

// respostaFixa responde sempre o mesmo status e corpo.
func respostaFixa(status int, corpo any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responderJSON(w, status, corpo)
	}
}

// transporteFunc é um http.RoundTripper feito de uma função.
type transporteFunc func(*http.Request) (*http.Response, error)

func (f transporteFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestCabecalhos(t *testing.T) {
	c, g := novoServidor(t, respostaFixa(http.StatusOK, map[string]any{"ok": true, "motor": "simulado", "ambiente": "sandbox"}))
	ctx := context.Background()

	st, err := c.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.OK || st.Motor != "simulado" || st.Ambiente != "sandbox" {
		t.Errorf("Status = %+v", st)
	}
	if _, err := c.Empresa.Obter(ctx); err != nil {
		t.Fatalf("Empresa.Obter: %v", err)
	}

	chamadas := g.lista()
	if len(chamadas) != 2 {
		t.Fatalf("esperava 2 chamadas, veio %d", len(chamadas))
	}
	if v := chamadas[0].cab.Get("Authorization"); v != "" {
		t.Errorf("Status não deve mandar Authorization; veio %q", v)
	}
	if v := chamadas[1].cab.Get("Authorization"); v != "Bearer "+chaveTeste {
		t.Errorf("Authorization = %q", v)
	}
	for _, ch := range chamadas {
		if v := ch.cab.Get("User-Agent"); v != "emissaoja-go/"+Versao {
			t.Errorf("User-Agent = %q", v)
		}
		if v := ch.cab.Get("Accept"); v != "application/json" {
			t.Errorf("Accept = %q", v)
		}
		if v := ch.cab.Get("Content-Type"); v != "" {
			t.Errorf("GET sem corpo não deve mandar Content-Type; veio %q", v)
		}
	}
}

func TestChaveLidaDoAmbienteNaPrimeiraChamada(t *testing.T) {
	t.Setenv("EMISSAOJA_KEY", "")
	c, g := novoServidor(t, respostaFixa(http.StatusOK, map[string]any{}), ComChave(""))
	ctx := context.Background()

	// A variável muda depois de criar o cliente: vale o valor lido na primeira chamada.
	t.Setenv("EMISSAOJA_KEY", "ej_test_do_ambiente")
	if _, err := c.Empresa.Obter(ctx); err != nil {
		t.Fatalf("Empresa.Obter: %v", err)
	}
	t.Setenv("EMISSAOJA_KEY", "ej_test_outra")
	if _, err := c.Empresa.Obter(ctx); err != nil {
		t.Fatalf("Empresa.Obter: %v", err)
	}
	for _, ch := range g.lista() {
		if v := ch.cab.Get("Authorization"); v != "Bearer ej_test_do_ambiente" {
			t.Errorf("Authorization = %q, esperava a chave lida na primeira chamada", v)
		}
	}
}

func TestSemChaveDevolveChaveInvalidaSemChamarAAPI(t *testing.T) {
	t.Setenv("EMISSAOJA_KEY", "")
	c, g := novoServidor(t, respostaFixa(http.StatusOK, map[string]any{"ok": true}), ComChave(""))
	ctx := context.Background()

	_, err := c.Empresa.Obter(ctx)
	var e *Erro
	if !errors.As(err, &e) {
		t.Fatalf("esperava *Erro, veio %v", err)
	}
	if e.Code != "chave_invalida" || e.Status != 0 {
		t.Errorf("erro = %v", e)
	}
	if n := len(g.lista()); n != 0 {
		t.Errorf("não devia chamar a API; houve %d chamadas", n)
	}
	if _, err := c.Status(ctx); err != nil {
		t.Errorf("Status não precisa de chave: %v", err)
	}
}

func TestEmitirComReferenciaInvalidaNaoChamaAAPI(t *testing.T) {
	c, g := novoServidor(t, respostaFixa(http.StatusAccepted, map[string]any{"pedido": map[string]any{"id": "p1"}}))
	invalidas := []string{"", "curta", "com espaco 123", "acentuação-123", strings.Repeat("a", 65), "quebra\nde-linha"}
	for _, ref := range invalidas {
		_, err := c.Notas.Emitir(context.Background(), ref, CamposNota{TomadorTipo: TomadorPJ, TomadorID: "tom-1", Valor: 10})
		if CodigoDoErro(err) != "chave_idempotencia_invalida" {
			t.Errorf("referência %q: esperava chave_idempotencia_invalida, veio %v", ref, err)
		}
	}
	if n := len(g.lista()); n != 0 {
		t.Errorf("não devia chamar a API; houve %d chamadas", n)
	}
}

func TestEmitirMandaIdempotencyKeyECorpoSemVazios(t *testing.T) {
	pedido := map[string]any{"id": "p1", "estado": "na_fila", "proxima_consulta_ms": 2000}
	previa := map[string]any{"resumo": map[string]any{"valor": 150.5}, "avisos": []any{}}
	c, g := novoServidor(t, respostaFixa(http.StatusAccepted, map[string]any{"pedido": pedido, "previa": previa}))

	r, err := c.Notas.Emitir(context.Background(), "pedido-0001", CamposNota{TomadorTipo: TomadorPJ, TomadorID: "tom-1", Valor: 150.5, Descricao: "Consultoria"})
	if err != nil {
		t.Fatalf("Emitir: %v", err)
	}
	if r.Pedido.ID != "p1" || r.Pedido.Estado != EstadoNaFila || r.Previa == nil || r.Repetido {
		t.Errorf("resposta = %+v", r)
	}
	if r.Pedido.ProximaConsultaMs == nil || *r.Pedido.ProximaConsultaMs != 2000 {
		t.Errorf("ProximaConsultaMs = %v", r.Pedido.ProximaConsultaMs)
	}

	ch := g.ultima(t)
	if ch.metodo != http.MethodPost || ch.uri != "/v1/notas" {
		t.Errorf("chamada = %s %s", ch.metodo, ch.uri)
	}
	if v := ch.cab.Get("Idempotency-Key"); v != "pedido-0001" {
		t.Errorf("Idempotency-Key = %q", v)
	}
	if v := ch.cab.Get("Content-Type"); v != "application/json" {
		t.Errorf("Content-Type = %q", v)
	}
	var corpo map[string]any
	if err := json.Unmarshal(ch.corpo, &corpo); err != nil {
		t.Fatalf("o corpo não é JSON: %v", err)
	}
	esperado := map[string]any{"tomador_tipo": "pj", "tomador_id": "tom-1", "valor": 150.5, "descricao": "Consultoria"}
	if len(corpo) != len(esperado) {
		t.Errorf("corpo = %v, esperava só %v", corpo, esperado)
	}
	for k, v := range esperado {
		if corpo[k] != v {
			t.Errorf("corpo[%q] = %v, esperava %v", k, corpo[k], v)
		}
	}
}

func TestEmitirMandaOpcionaisQuandoInformados(t *testing.T) {
	c, g := novoServidor(t, respostaFixa(http.StatusOK, map[string]any{"pedido": map[string]any{"id": "p1", "estado": "autorizada"}, "repetido": true}))

	campos := CamposNota{TomadorTipo: TomadorPF, TomadorID: "tom-2", Valor: 99.9, RetemISS: Ptr(false), AliquotaISS: 2.5, ConfirmoRepeticao: true, PermitirRepetidaDe: "p0"}
	r, err := c.Notas.Emitir(context.Background(), "pedido_0002", campos)
	if err != nil {
		t.Fatalf("Emitir: %v", err)
	}
	if !r.Repetido {
		t.Errorf("Repetido = false, esperava true")
	}
	var corpo map[string]any
	if err := json.Unmarshal(g.ultima(t).corpo, &corpo); err != nil {
		t.Fatalf("o corpo não é JSON: %v", err)
	}
	esperado := map[string]any{"tomador_tipo": "pf", "tomador_id": "tom-2", "valor": 99.9, "retem_iss": false, "aliquota_iss": 2.5, "confirmo_repeticao": true, "permitir_repetida_de": "p0"}
	if len(corpo) != len(esperado) {
		t.Errorf("corpo = %v, esperava %v", corpo, esperado)
	}
	for k, v := range esperado {
		if corpo[k] != v {
			t.Errorf("corpo[%q] = %v, esperava %v", k, corpo[k], v)
		}
	}
}

func TestErroNotaBloqueadaTrazBloqueiosExtrasERequestID(t *testing.T) {
	bloqueio := map[string]any{"code": "tomador_incompleto", "mensagem": "Complete o endereço do cliente.", "acao": "completar_tomador"}
	corpo := map[string]any{"status": "ERROR", "code": "nota_bloqueada", "message": "Antes de emitir, resolva:", "bloqueios": []any{bloqueio}}
	c, _ := novoServidor(t, respostaFixa(http.StatusUnprocessableEntity, corpo))

	_, err := c.Notas.Emitir(context.Background(), "pedido-0003", CamposNota{TomadorTipo: TomadorPJ, TomadorID: "tom-1", Valor: 10})
	var e *Erro
	if !errors.As(err, &e) {
		t.Fatalf("esperava *Erro, veio %v", err)
	}
	if e.Status != 422 || e.Code != "nota_bloqueada" || e.Message != "Antes de emitir, resolva:" {
		t.Errorf("erro = %d %s %q", e.Status, e.Code, e.Message)
	}
	if e.RequestID != "req-teste" {
		t.Errorf("RequestID = %q", e.RequestID)
	}
	for _, k := range []string{"status", "code", "message"} {
		if _, ok := e.Extras[k]; ok {
			t.Errorf("Extras não devia ter %q", k)
		}
	}
	if _, ok := e.Extras["bloqueios"]; !ok {
		t.Errorf("Extras sem bloqueios: %v", e.Extras)
	}
	bs := e.Bloqueios()
	if len(bs) != 1 || bs[0].Code != "tomador_incompleto" || bs[0].Acao != "completar_tomador" || bs[0].Mensagem == "" {
		t.Errorf("Bloqueios() = %+v", bs)
	}
	if CodigoDoErro(err) != "nota_bloqueada" {
		t.Errorf("CodigoDoErro = %q", CodigoDoErro(err))
	}
	if msg := err.Error(); !strings.Contains(msg, "nota_bloqueada") || !strings.Contains(msg, "422") || !strings.Contains(msg, "req-teste") {
		t.Errorf("Error() = %q", msg)
	}
	if (&Erro{}).Bloqueios() != nil {
		t.Errorf("Bloqueios() sem bloqueios devia ser nil")
	}
}

func TestErroComCorpoQueNaoEJSON(t *testing.T) {
	casos := []struct {
		nome     string
		corpo    string
		mensagem string
	}{
		{"html", "<html>Bad Gateway</html>", "<html>Bad Gateway</html>"},
		{"vazio", "", "HTTP 502"},
		{"longo", strings.Repeat("é", 250), strings.Repeat("é", 200)},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			corpo := caso.corpo
			c, _ := novoServidor(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				w.WriteHeader(http.StatusBadGateway)
				_, _ = io.WriteString(w, corpo)
			})
			_, err := c.Empresa.Obter(context.Background())
			var e *Erro
			if !errors.As(err, &e) {
				t.Fatalf("esperava *Erro, veio %v", err)
			}
			if e.Status != http.StatusBadGateway || e.Code != "erro_desconhecido" || e.Message != caso.mensagem {
				t.Errorf("erro = %d %s %q", e.Status, e.Code, e.Message)
			}
		})
	}
}

func TestRespostaDeSucessoForaDoFormato(t *testing.T) {
	c, _ := novoServidor(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "<html>manutenção</html>")
	})
	_, err := c.Empresa.Obter(context.Background())
	var e *Erro
	if !errors.As(err, &e) || e.Code != "erro_desconhecido" || e.Status != http.StatusOK {
		t.Fatalf("esperava erro_desconhecido com HTTP 200, veio %v", err)
	}
}

func TestAguardarRespeitaProximaConsulta(t *testing.T) {
	pedidos := []map[string]any{
		{"id": "p1", "estado": "na_fila", "proxima_consulta_ms": 1500},
		{"id": "p1", "estado": "emitindo", "proxima_consulta_ms": 200},
		{"id": "p1", "estado": "confirmando", "proxima_consulta_ms": nil},
		{"id": "p1", "estado": "autorizada", "numero": "42", "nota": map[string]any{"origem": "local", "id": "n1"}},
	}
	var mu sync.Mutex
	vez := 0
	c, g := novoServidor(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		p := pedidos[vez]
		if vez < len(pedidos)-1 {
			vez++
		}
		mu.Unlock()
		responderJSON(w, http.StatusOK, map[string]any{"pedido": p})
	})
	var esperas []time.Duration
	c.dormir = func(ctx context.Context, d time.Duration) error {
		esperas = append(esperas, d)
		return nil
	}

	r, err := c.Notas.Aguardar(context.Background(), "p1", &AguardarOpcoes{Intervalo: time.Second})
	if err != nil {
		t.Fatalf("Aguardar: %v", err)
	}
	if r.Pedido.Estado != EstadoAutorizada || r.Pedido.Numero != "42" || r.Pedido.Nota == nil || r.Pedido.Nota.Origem != OrigemLocal {
		t.Errorf("pedido = %+v", r.Pedido)
	}
	// 1500 ms vêm da API; 200 ms sobem para o mínimo de 1 s; sem proxima_consulta_ms, 3 s.
	esperado := []time.Duration{1500 * time.Millisecond, time.Second, 3 * time.Second}
	if len(esperas) != len(esperado) {
		t.Fatalf("esperas = %v, esperava %v", esperas, esperado)
	}
	for i := range esperado {
		if esperas[i] != esperado[i] {
			t.Errorf("espera %d = %v, esperava %v", i, esperas[i], esperado[i])
		}
	}
	if n := len(g.lista()); n != 4 {
		t.Errorf("esperava 4 consultas, veio %d", n)
	}
	if uri := g.ultima(t).uri; uri != "/v1/emissoes/p1" {
		t.Errorf("uri = %q", uri)
	}
}

func TestAguardarTempoEsgotadoTrazPedidoID(t *testing.T) {
	c, g := novoServidor(t, respostaFixa(http.StatusOK, map[string]any{"pedido": map[string]any{"id": "p9", "estado": "emitindo", "proxima_consulta_ms": 2000}}))
	agora := time.Date(2026, time.November, 5, 12, 0, 0, 0, time.UTC)
	c.agora = func() time.Time { return agora }
	c.dormir = func(ctx context.Context, d time.Duration) error {
		agora = agora.Add(d)
		return nil
	}

	_, err := c.Notas.Aguardar(context.Background(), "p9", &AguardarOpcoes{Timeout: 5 * time.Second})
	var e *Erro
	if !errors.As(err, &e) || e.Code != "tempo_esgotado" || e.Status != 0 {
		t.Fatalf("esperava tempo_esgotado, veio %v", err)
	}
	if e.Extras["pedido_id"] != "p9" {
		t.Errorf("Extras = %v, esperava pedido_id p9", e.Extras)
	}
	// Consultas em 0 s, 2 s, 4 s e 6 s; a de 6 s já passou do prazo de 5 s.
	if n := len(g.lista()); n != 4 {
		t.Errorf("esperava 4 consultas, veio %d", n)
	}
}

func TestAguardarParaQuandoOContextoECancelado(t *testing.T) {
	c, _ := novoServidor(t, respostaFixa(http.StatusOK, map[string]any{"pedido": map[string]any{"id": "p1", "estado": "emitindo", "proxima_consulta_ms": 60000}}))
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	c.dormir = func(ctxEspera context.Context, d time.Duration) error {
		cancelar() // cancela no começo da espera de 60 s
		return dormirComContexto(ctxEspera, d)
	}

	inicio := time.Now()
	_, err := c.Notas.Aguardar(ctx, "p1", nil)
	if CodigoDoErro(err) != "cancelado" || !errors.Is(err, context.Canceled) {
		t.Fatalf("esperava cancelado, veio %v", err)
	}
	if time.Since(inicio) > 10*time.Second {
		t.Errorf("Aguardar demorou %v para notar o cancelamento", time.Since(inicio))
	}
	if err := dormirComContexto(context.Background(), time.Millisecond); err != nil {
		t.Errorf("dormirComContexto sem cancelamento: %v", err)
	}
}

func TestPDFDevolveBytes(t *testing.T) {
	pdf := []byte("%PDF-1.7\n\x00\xff\xfe conteúdo binário")
	c, g := novoServidor(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(pdf)
	})

	recebido, err := c.Notas.PDF(context.Background(), OrigemLocal, "nota-1")
	if err != nil {
		t.Fatalf("PDF: %v", err)
	}
	if !bytes.Equal(recebido, pdf) {
		t.Errorf("PDF = %q, esperava %q", recebido, pdf)
	}
	if ch := g.ultima(t); ch.metodo != http.MethodGet || ch.uri != "/v1/notas/local/nota-1/pdf" {
		t.Errorf("chamada = %s %s", ch.metodo, ch.uri)
	}
}

func TestPDFComErroDaAPI(t *testing.T) {
	c, _ := novoServidor(t, respostaFixa(http.StatusNotFound, map[string]any{"status": "ERROR", "code": "nota_nao_encontrada", "message": "Nota não encontrada."}))
	pdf, err := c.Notas.PDF(context.Background(), OrigemImportada, "nota-x")
	if CodigoDoErro(err) != "nota_nao_encontrada" || pdf != nil {
		t.Errorf("esperava nota_nao_encontrada, veio %v (%d bytes)", err, len(pdf))
	}
}

func TestXMLDevolveTexto(t *testing.T) {
	const xml = `<?xml version="1.0" encoding="UTF-8"?><NFSe><xMun>São Paulo</xMun></NFSe>`
	c, g := novoServidor(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = io.WriteString(w, xml)
	})

	texto, err := c.Notas.XML(context.Background(), OrigemImportada, "nota-2")
	if err != nil {
		t.Fatalf("XML: %v", err)
	}
	if texto != xml {
		t.Errorf("XML = %q", texto)
	}
	if uri := g.ultima(t).uri; uri != "/v1/notas/importada/nota-2/xml" {
		t.Errorf("uri = %q", uri)
	}
}

func TestQueryOmiteParametrosVazios(t *testing.T) {
	c, g := novoServidor(t, respostaFixa(http.StatusOK, map[string]any{}))
	ctx := context.Background()

	if _, err := c.Tomadores.Listar(ctx, &FiltroTomadores{Busca: "alfa", Pagina: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Tomadores.Buscar(ctx, "maria silva"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Notas.Listar(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Notas.Listar(ctx, &FiltroNotas{Periodo: "personalizado", De: "2026-10-01", Ate: "2026-10-31", Limite: 50}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Servicos.Listar(ctx, &FiltroServicos{}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Servicos.Catalogo(ctx, "consultoria", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Emissoes.Listar(ctx, &FiltroEmissoes{Estados: []EstadoPedido{EstadoIncerta, EstadoNaFila}}); err != nil {
		t.Fatal(err)
	}

	esperado := []string{
		"/v1/tomadores?busca=alfa&pagina=2",
		"/v1/tomadores/busca?q=maria+silva",
		"/v1/notas",
		"/v1/notas?ate=2026-10-31&de=2026-10-01&limite=50&periodo=personalizado",
		"/v1/servicos",
		"/v1/catalogo/servicos?q=consultoria",
		"/v1/emissoes?estados=incerta%2Cna_fila",
	}
	chamadas := g.lista()
	if len(chamadas) != len(esperado) {
		t.Fatalf("esperava %d chamadas, veio %d", len(esperado), len(chamadas))
	}
	for i, uri := range esperado {
		if chamadas[i].uri != uri {
			t.Errorf("chamada %d = %q, esperava %q", i, chamadas[i].uri, uri)
		}
	}
}

func TestSegmentosDoCaminhoCodificados(t *testing.T) {
	c, g := novoServidor(t, respostaFixa(http.StatusOK, map[string]any{}))
	ctx := context.Background()

	if _, err := c.Tomadores.Obter(ctx, TomadorPJ, "a/b c?d"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Servicos.Desativar(ctx, "x/y"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Operacoes.Obter(ctx, "op 1"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Emissoes.Conferir(ctx, "p#1"); err != nil {
		t.Fatal(err)
	}

	esperado := []string{
		"GET /v1/tomadores/pj/a%2Fb%20c%3Fd",
		"DELETE /v1/servicos/x%2Fy",
		"GET /v1/operacoes/op%201",
		"POST /v1/emissoes/p%231/conferir",
	}
	chamadas := g.lista()
	if len(chamadas) != len(esperado) {
		t.Fatalf("esperava %d chamadas, veio %d", len(esperado), len(chamadas))
	}
	for i, linha := range esperado {
		if obtido := chamadas[i].metodo + " " + chamadas[i].uri; obtido != linha {
			t.Errorf("chamada %d = %q, esperava %q", i, obtido, linha)
		}
	}
}

func TestCNPJConsultarTiraAPontuacao(t *testing.T) {
	c, g := novoServidor(t, respostaFixa(http.StatusOK, map[string]any{"cnpj": "11222333000181", "razao_social": "EMPRESA DE TESTE LTDA"}))

	doc, err := c.CNPJ.Consultar(context.Background(), "11.222.333/0001-81")
	if err != nil {
		t.Fatalf("Consultar: %v", err)
	}
	if doc["razao_social"] != "EMPRESA DE TESTE LTDA" {
		t.Errorf("documento = %v", doc)
	}
	if uri := g.ultima(t).uri; uri != "/v1/cnpj/11222333000181" {
		t.Errorf("uri = %q", uri)
	}
}

func TestCorposDeTomadoresServicosEEmpresa(t *testing.T) {
	c, g := novoServidor(t, respostaFixa(http.StatusOK, map[string]any{}))
	ctx := context.Background()

	if _, err := c.Tomadores.Criar(ctx, "12345678000195", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Tomadores.Criar(ctx, "52998224725", "Maria da Silva"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Tomadores.Atualizar(ctx, TomadorPF, "tom-2", Campos{"email": "maria@exemplo.com.br", "complemento": nil}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Servicos.Criar(ctx, CamposServico{Descricao: "Consultoria", CTribNac: "171901", TribISSQN: "tributavel"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Servicos.Atualizar(ctx, "srv-1", Campos{"aliquota_iss": 2.5}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Servicos.Desativar(ctx, "srv-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Empresa.Fiscal(ctx, CamposFiscais{AliquotaSimples: 6}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Empresa.RelerFisco(ctx); err != nil {
		t.Fatal(err)
	}

	esperado := []struct {
		metodo string
		uri    string
		corpo  string
	}{
		{http.MethodPost, "/v1/tomadores", `{"documento":"12345678000195"}`},
		{http.MethodPost, "/v1/tomadores", `{"documento":"52998224725","nome":"Maria da Silva"}`},
		{http.MethodPut, "/v1/tomadores/pf/tom-2", `{"complemento":null,"email":"maria@exemplo.com.br"}`},
		{http.MethodPost, "/v1/servicos", `{"descricao":"Consultoria","c_trib_nac":"171901","trib_issqn":"tributavel","retem_iss":false}`},
		{http.MethodPut, "/v1/servicos/srv-1", `{"aliquota_iss":2.5}`},
		{http.MethodDelete, "/v1/servicos/srv-1", ""},
		{http.MethodPut, "/v1/empresa/fiscal", `{"aliquota_simples":6}`},
		{http.MethodPost, "/v1/empresa/reler-fisco", ""},
	}
	chamadas := g.lista()
	if len(chamadas) != len(esperado) {
		t.Fatalf("esperava %d chamadas, veio %d", len(esperado), len(chamadas))
	}
	for i, e := range esperado {
		ch := chamadas[i]
		if ch.metodo != e.metodo || ch.uri != e.uri || string(ch.corpo) != e.corpo {
			t.Errorf("chamada %d = %s %s %s; esperava %s %s %s", i, ch.metodo, ch.uri, ch.corpo, e.metodo, e.uri, e.corpo)
		}
		temCorpo := e.corpo != ""
		if ct := ch.cab.Get("Content-Type"); (ct == "application/json") != temCorpo {
			t.Errorf("chamada %d: Content-Type = %q com corpo %q", i, ct, e.corpo)
		}
	}
}

func TestConectarEAguardarCredencial(t *testing.T) {
	var mu sync.Mutex
	consultas := 0
	c, g := novoServidor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			responderJSON(w, http.StatusAccepted, map[string]any{"tentativa": map[string]any{"id": "t1", "estado": "verificando", "proximo_passo": "aguardar"}})
			return
		}
		mu.Lock()
		consultas++
		passo := "aguardar"
		if consultas >= 3 {
			passo = "pronto"
		}
		mu.Unlock()
		tentativa := map[string]any{"id": "t1", "estado": "aceita", "proximo_passo": passo}
		responderJSON(w, http.StatusOK, map[string]any{"credencial": map[string]any{"estado": "ativa"}, "onboarding": map[string]any{"estado": "pronto"}, "tentativa": tentativa})
	})
	var esperas []time.Duration
	c.dormir = func(ctx context.Context, d time.Duration) error {
		esperas = append(esperas, d)
		return nil
	}
	ctx := context.Background()

	con, err := c.Empresa.Conectar(ctx, CamposConexao{CNPJ: "11111111000191", Senha: "qualquer-senha"})
	if err != nil {
		t.Fatalf("Conectar: %v", err)
	}
	if con.Tentativa.ID != "t1" || con.Tentativa.ProximoPasso != "aguardar" {
		t.Errorf("tentativa = %+v", con.Tentativa)
	}
	put := g.ultima(t)
	if put.metodo != http.MethodPut || put.uri != "/v1/empresa/credencial" {
		t.Errorf("chamada = %s %s", put.metodo, put.uri)
	}
	if string(put.corpo) != `{"cnpj":"11111111000191","senha":"qualquer-senha","ciente_ultima_tentativa":false}` {
		t.Errorf("corpo = %s", put.corpo)
	}

	fim, err := c.Empresa.AguardarCredencial(ctx, nil)
	if err != nil {
		t.Fatalf("AguardarCredencial: %v", err)
	}
	if fim.Tentativa == nil || fim.Tentativa.ProximoPasso != "pronto" || fim.Credencial.Estado != "ativa" || fim.Onboarding.Estado != "pronto" {
		t.Errorf("credencial = %+v", fim)
	}
	if len(esperas) != 2 || esperas[0] != 2*time.Second || esperas[1] != 2*time.Second {
		t.Errorf("esperas = %v, esperava duas de 2 s", esperas)
	}
}

func TestCancelarEAguardarOperacao(t *testing.T) {
	var mu sync.Mutex
	consultas := 0
	c, g := novoServidor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			responderJSON(w, http.StatusAccepted, map[string]any{"operacao": map[string]any{"id": "op1", "tipo": "cancelamento", "estado": "processando"}})
			return
		}
		mu.Lock()
		consultas++
		estado := "processando"
		if consultas >= 2 {
			estado = "concluida"
		}
		mu.Unlock()
		operacao := map[string]any{"id": "op1", "tipo": "cancelamento", "estado": estado, "resultado": map[string]any{"situacao": "cancelada"}}
		responderJSON(w, http.StatusOK, map[string]any{"operacao": operacao})
	})
	c.dormir = func(ctx context.Context, d time.Duration) error { return nil }
	ctx := context.Background()

	op, err := c.Notas.Cancelar(ctx, OrigemLocal, "nota-1", MotivoErroNaEmissao, "")
	if err != nil {
		t.Fatalf("Cancelar: %v", err)
	}
	if op.Operacao.ID != "op1" || op.Operacao.Estado != "processando" {
		t.Errorf("operação = %+v", op.Operacao)
	}
	if ch := g.ultima(t); ch.uri != "/v1/notas/local/nota-1/cancelar" || string(ch.corpo) != `{"motivo":"1"}` {
		t.Errorf("chamada = %s %s %s", ch.metodo, ch.uri, ch.corpo)
	}
	if _, err := c.Notas.Cancelar(ctx, OrigemImportada, "nota-2", MotivoServicoNaoPrestado, "O serviço não foi prestado ao cliente"); err != nil {
		t.Fatalf("Cancelar com justificativa: %v", err)
	}
	if ch := g.ultima(t); string(ch.corpo) != `{"justificativa":"O serviço não foi prestado ao cliente","motivo":"2"}` {
		t.Errorf("corpo = %s", ch.corpo)
	}

	fim, err := c.Operacoes.Aguardar(ctx, op.Operacao.ID, &AguardarOpcoes{Intervalo: time.Millisecond})
	if err != nil {
		t.Fatalf("Operacoes.Aguardar: %v", err)
	}
	if fim.Operacao.Estado != "concluida" || fim.Operacao.Resultado["situacao"] != "cancelada" {
		t.Errorf("operação = %+v", fim.Operacao)
	}
	if uri := g.ultima(t).uri; uri != "/v1/operacoes/op1" {
		t.Errorf("uri = %q", uri)
	}
}

func TestFalhaDeRedeViraErroRede(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	base := srv.URL
	srv.Close()

	c := NovoCliente(ComChave(chaveTeste), ComBaseURL(base))
	_, err := c.Empresa.Obter(context.Background())
	var e *Erro
	if !errors.As(err, &e) || e.Code != "rede" || e.Status != 0 {
		t.Fatalf("esperava rede, veio %v", err)
	}
	if errors.Unwrap(err) == nil {
		t.Errorf("o erro de rede devia trazer a causa")
	}
}

func TestTimeoutDaChamadaViraTempoEsgotado(t *testing.T) {
	c, _ := novoServidor(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}, ComTimeout(50*time.Millisecond))

	_, err := c.Empresa.Obter(context.Background())
	if CodigoDoErro(err) != "tempo_esgotado" || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("esperava tempo_esgotado, veio %v", err)
	}
}

func TestComHTTPClientUsaOTransporteInjetado(t *testing.T) {
	var recebida *http.Request
	transporte := transporteFunc(func(r *http.Request) (*http.Response, error) {
		recebida = r
		corpo := io.NopCloser(strings.NewReader(`{"ok":true,"versao":"0.1.0","motor":"ok","ambiente":"producao"}`))
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: corpo, Request: r}, nil
	})
	c := NovoCliente(ComHTTPClient(&http.Client{Transport: transporte}))

	st, err := c.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.OK || st.Ambiente != "producao" {
		t.Errorf("status = %+v", st)
	}
	if recebida == nil || recebida.URL.String() != BaseProducao+"/v1/status" {
		t.Fatalf("requisição = %v", recebida)
	}
	if v := recebida.Header.Get("User-Agent"); v != "emissaoja-go/"+Versao {
		t.Errorf("User-Agent = %q", v)
	}
}

func TestPadroesDoClienteECodigoDoErro(t *testing.T) {
	if b := NovoCliente().BaseURL(); b != BaseProducao {
		t.Errorf("BaseURL padrão = %q", b)
	}
	if b := NovoCliente(ComBaseURL(BaseSandbox + "/")).BaseURL(); b != BaseSandbox {
		t.Errorf("BaseURL com barra no fim = %q", b)
	}
	if b := NovoCliente(ComBaseURL("  ")).BaseURL(); b != BaseProducao {
		t.Errorf("BaseURL vazia = %q", b)
	}
	if c := NovoCliente(); c.timeout != 30*time.Second {
		t.Errorf("timeout padrão = %v", c.timeout)
	}
	if c := NovoCliente(ComTimeout(-time.Second)); c.timeout != 0 {
		t.Errorf("timeout negativo = %v, esperava 0", c.timeout)
	}
	if CodigoDoErro(nil) != "" || CodigoDoErro(errors.New("outro erro")) != "" {
		t.Errorf("CodigoDoErro de erro que não é *Erro devia ser vazio")
	}
	embrulhado := fmt.Errorf("emitindo a nota: %w", novoErro("rede", "falhou"))
	if CodigoDoErro(embrulhado) != "rede" {
		t.Errorf("CodigoDoErro(embrulhado) = %q", CodigoDoErro(embrulhado))
	}
	if !EstadoIncerta.Terminal() || EstadoConfirmando.Terminal() {
		t.Errorf("Terminal() errado")
	}
	if p := Ptr(false); p == nil || *p {
		t.Errorf("Ptr(false) = %v", p)
	}
}
