# CLAUDE.md — grafia: Índice Estruturado de Código

Este projeto usa `grafia` (índice SQLite de símbolos, referências e dependências).

## Regra principal

Para localizar um **símbolo** (função, método, tipo, campo) e ver seu código:

```bash
grafia grep <nome>                    # declaração + todos os usos, COM o código-fonte
grafia grep <nome> --kind call        # só chamadas
grafia grep <nome> -C 3               # com linhas de contexto
grafia grep <nome> --max 60           # mais resultados (default 40)
```

**Símbolos homônimos? Use padrão pontuado** — seleciona só o que você quer, sem regex de receiver:

```bash
grafia grep db.New          # a New() do pacote db (não as dos outros pacotes)
grafia grep Router.Get      # o Get() do tipo Router (não o dos outros tipos)
grafia grep internal/db.New # desambigua dois pacotes chamados db
```

Os padrões pontuados funcionam também em `query symbol`, `query callers`, `query callees` e `query usages`.

Uma chamada devolve a declaração completa e cada uso com `arquivo:linha` e a linha de código — não é preciso ler os arquivos para reconhecimento. O `grafia grep` detecta arquivos alterados e reindexa sozinho antes de responder; **não** é necessário rodar `grafia update` após cada edição.

Use grep/rg comum apenas para **texto literal** (strings, mensagens de erro, comentários).

## Outros comandos (quando precisar)

```bash
grafia query callers <nome>             # quem chama (JSON)
grafia query callees <nome>             # o que chama (JSON)
grafia query file <path>                # símbolos do arquivo (JSON)
grafia query deps <path> [--transitive] # imports (JSON)
grafia stats                            # visão geral do projeto
grafia sql "<query>"                    # SQL direto: files, symbols, refs, dependencies
grafia update --git-diff                # sincronizar após pull/merge externo
```

## Notas

- O banco fica em `.grafia.db` (gitignored). Não edite esse arquivo.
- Ao final de uma tarefa com muitas edições, rode `grafia update <arquivos editados>` uma única vez (em lote).
- Não é preciso verificar o índice após editar — ele se autocorrige na próxima consulta.
