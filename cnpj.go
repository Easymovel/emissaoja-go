package emissaoja

import (
	"context"
	"strings"
)

// RecursoCNPJ consulta os dados cadastrais de um CNPJ na Receita Federal. Use pelo campo Cliente.CNPJ.
type RecursoCNPJ struct {
	c *Cliente
}

// Consultar devolve os dados cadastrais do CNPJ (GET /v1/cnpj/{cnpj}): razão social, situação, CNAEs,
// endereço, Simples, MEI e sócios, em formato livre. A pontuação é retirada antes da chamada. Não cria
// tomador; serve para conferir antes de cadastrar. Limite: 30 consultas por minuto por chave.
func (r *RecursoCNPJ) Consultar(ctx context.Context, cnpj string) (map[string]any, error) {
	return chamarMapa(ctx, r.c, get("/v1/cnpj/"+segmento(soDigitos(cnpj)), nil))
}

// soDigitos tira tudo o que não é dígito: "11.222.333/0001-81" vira "11222333000181".
func soDigitos(s string) string {
	var b strings.Builder
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			b.WriteRune(ch)
		}
	}
	return b.String()
}
