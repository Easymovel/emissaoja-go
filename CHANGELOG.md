# Changelog

Todas as mudanças relevantes deste módulo. Formato baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/); versões seguem [SemVer](https://semver.org/lang/pt-BR/). No Go, cada versão é uma tag `vX.Y.Z` deste repositório.

## [0.1.0] - 2026-11-05

Primeira versão pública.

### Adicionado

- Módulo `emissaoja.com.br/go` (pacote `emissaoja`), sem dependências, para Go 1.21 ou mais novo.
- `NovoCliente` com as opções `ComChave`, `ComBaseURL`, `ComHTTPClient` e `ComTimeout` (padrão 30 s); constantes `BaseProducao` e `BaseSandbox`; a chave sai de `EMISSAOJA_KEY` na primeira chamada quando não é informada.
- Recursos da API v1: `Empresa` (`Obter`, `Conectar`, `Credencial`, `AguardarCredencial`, `Onboarding`, `Fiscal`, `RelerFisco`), `Tomadores` (`Listar`, `Buscar`, `Criar`, `Obter`, `Atualizar`), `Servicos` (`Listar`, `Criar`, `Atualizar`, `Desativar`, `Catalogo`), `Notas` (`Emitir`, `Aguardar`, `Listar`, `PDF`, `XML`, `Cancelar`), `Emissoes` (`Obter`, `Listar`, `Conferir`), `Operacoes` (`Obter`, `Aguardar`), `CNPJ` (`Consultar`) e `Status`.
- `Notas.Emitir` com referência obrigatória enviada como `Idempotency-Key` e validada antes de chamar a API.
- `Notas.Aguardar`, `Operacoes.Aguardar` e `Empresa.AguardarCredencial`, que respeitam `proxima_consulta_ms` e o cancelamento do contexto.
- `*Erro` com `Status`, `Code`, `Message`, `Extras` e `RequestID`; `CodigoDoErro`, `Erro.Bloqueios` e suporte a `errors.As` e `errors.Is`.
- Tipos com tags JSON para pedido, nota, tomador, serviço, operação, tentativa, credencial e as respostas da API.

[0.1.0]: https://github.com/Easymovel/emissaoja-go/releases/tag/v0.1.0
