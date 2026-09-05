# Changelog

## Autocomplete com gopls

### Novos arquivos

| Arquivo | Descrição |
|---|---|
| `gopls.go` | Cliente LSP que inicia o `gopls serve` via stdio e expõe sync de documentos e completions |
| `gopls_integration.go` | Integração do gopls com o editor (ciclo de vida, sincronização e busca de sugestões) |
| `lsp_position.go` | Conversão de posições byte ↔ UTF-16 exigida pelo protocolo LSP |
| `completion_popup.go` | Camada visual de sugestões dentro do editor (sem overlay de canvas) |

### Alterações

| Arquivo | Descrição |
|---|---|
| `code_editor.go` | Renderer customizado (`codeEditorRenderer`), completions inline, filtro por prefixo e aceite com Enter/Tab |
| `main.go` | Inicia o gopls ao abrir pasta/arquivo `.go` e sincroniza o documento a cada alteração |
| `settings.go` | Reinicia o gopls ao alterar o GOROOT em Propriedades |
| `go.mod` | Adiciona `go.lsp.dev/protocol`, `go.lsp.dev/jsonrpc2` e `go.lsp.dev/uri` |

### Cliente LSP (`gopls.go`)

- Comunicação JSON-RPC via `go.lsp.dev/protocol` (sem montar mensagens manualmente)
- `initialize` / `initialized` com a pasta do projeto como workspace
- Sincronização de documentos com `didOpen` / `didChange` (sync completo)
- Consulta `textDocument/completion` e converte `TextEdit` / `InsertText` em sugestões aplicáveis
- Resolve o binário `gopls` com `resolveToolBinary()` (mesmo mecanismo do `goimports`)

### Lista de sugestões (`completion_popup.go`)

- Renderizada **dentro do editor** com `container.WithoutLayout`, sem `widget.PopUp`
- Painel compacto de **6 linhas** com scroll interno; até 50 itens na lista
- Linhas com `canvas.Text` monoespaçado e texto truncado em 72 caracteres
- Não rouba foco — é possível continuar digitando com a lista visível (ex.: `Print` → `Println`)
- Filtro local em tempo real pelo prefixo do identificador sendo digitado
- Posicionada abaixo da linha do cursor, alinhada à coluna no `TextGrid`
- Item selecionado destacado com fundo colorido e negrito

### Renderer do editor (`code_editor.go`)

- `codeEditorRenderer` posiciona manualmente o scroll (tela inteira) e o painel de sugestões (tamanho fixo)
- Evita o `StackLayout` do Fyne, que redimensionava a lista para cobrir todo o editor

### Atalhos e gatilhos

| Ação | Comportamento |
|---|---|
| **Ctrl+Space** | Dispara completion manualmente |
| Digitar `.` | Abre sugestões após ponto (ex.: `fmt.`) |
| Digitar identificador | Filtra a lista localmente; reconsulta o gopls com debounce (120–200 ms) |
| **↑** / **↓** | Navega na lista sem mover o cursor do editor |
| **Enter** / **Tab** | Aceita o item selecionado |
| **Esc** | Fecha a lista sem alterar o texto |

### Problemas corrigidos

**Lista bloqueava a digitação:** o `widget.PopUp` adiciona um overlay no canvas do Fyne que desvia o foco do teclado. A lista foi movida para uma camada interna do editor, mantendo o foco no `codeEditor`.

**Posicionamento incorreto:** a lista era ancorada no editor inteiro; agora usa a posição do `TextGrid` (gutter de linhas + coluna do cursor).

**Lista cobria todo o código:** o `StackLayout` redimensionava a camada de sugestões para o tamanho inteiro do editor a cada refresh, exibindo dezenas de itens esticados. Corrigido com `codeEditorRenderer` + `WithoutLayout` e painel de altura fixa com scroll.

### Requisitos

- `gopls` instalado (`go install golang.org/x/tools/gopls@latest`)
- Pasta de projeto aberta (**Arquivo → Abrir pasta...**) ou arquivo `.go` carregado

---

## Realce de sintaxe Go no editor

### Novos arquivos

| Arquivo | Descrição |
|---|---|
| `highlight.go` | Parser de sintaxe Go com coloração por token |
| `highlight_test.go` | Testes do realce de sintaxe |

### Alterações

| Arquivo | Descrição |
|---|---|
| `code_editor.go` | Integra `goSyntaxHighlight` junto aos rainbow brackets |

### Paleta de sintaxe

| Elemento | Cor | Exemplos |
|---|---|---|
| Funções | `#61afef` (azul) | `Hello`, `Println` |
| Structs/tipos | `#e5c07b` (amarelo) | `User`, tipos declarados com `type` |
| Packages | `#56b6c2` (ciano) | `main`, paths e aliases em `import` |
| Keywords | `#c678dd` (roxo) | `func`, `type`, `struct`, `import` |
| Strings | `#98c379` (verde) | `"texto"`, `` `raw` `` |
| Comentários | `#5c6370` (cinza) | `//` e `/* */` |
| Números | `#d19a66` (laranja) | `42`, `0xFF`, `3.14` |

### Detecção

- Declarações `func Nome()` e métodos `func (r *T) Nome()`
- Chamadas `pkg.Funcao()` (identificador após `.`)
- Declarações `type Nome struct` e reuso do tipo no código
- `package main` e paths/aliases em blocos `import`
- Ignora strings e comentários (mesma lógica dos rainbow brackets)
- Rainbow brackets mantêm prioridade sobre a sintaxe nos caracteres `()`, `[]` e `{}`

---

## Correção da execução no terminal

### Alterações

| Arquivo | Descrição |
|---|---|
| `run.go` | Execução via `exec.Command` em vez de comando no shell; `ensureTerminalOpen` sincroniza o painel antes de rodar |
| `terminal_panel.go` | `whenReady` aguarda o shell ficar pronto; `runGoFile` e `displayOutput` exibem só a saída do programa (sem prompts `>` de heredoc) |

### Problemas corrigidos

**Execução exigia dois cliques:** ao pressionar F5 com o terminal fechado, o painel abria mas o comando era enviado antes do shell estar pronto (layout e PTY ainda não inicializados). O comando falhava silenciosamente e era preciso executar de novo.

**Comando visível no terminal:** o `go run` era digitado no shell interativo e aparecia ecoado junto com a saída.

**Prompts `>` antes da saída:** o script de exibição usava heredoc (`base64 -d <<'...'`), e o bash interativo mostrava dois prompts de continuação (PS2) — um por linha do heredoc — antes de imprimir o resultado do programa.

### Solução

1. `ensureTerminalOpen` abre o painel, cria a aba se necessário e força refresh do layout
2. `whenReady` aguarda o terminal aceitar entrada (até ~10 s) antes de executar
3. `go run` é executado diretamente com `exec.Command`, capturando stdout e stderr
4. Apenas o resultado é exibido no terminal (com `stty -echo` + `printf | base64 -d` em uma linha no Linux, evitando heredoc e prompts PS2)

---

## Tema Rainbow Brackets e editor com coloração

### Novos arquivos

| Arquivo | Descrição |
|---|---|
| `theme.go` | Tema escuro customizado para Fyne, inspirado em editores como One Dark Pro |
| `brackets.go` | Parser de rainbow brackets com suporte a strings e comentários Go |
| `brackets_test.go` | Testes do parser de brackets |
| `code_editor.go` | Editor baseado em `TextGrid` com coloração de brackets e cursor |
| `format.go` | Formatação de código Go via `goimports` |
| `format_test.go` | Testes de formatação |
| `run.go` | Execução do arquivo `.go` atual no terminal integrado |
| `settings.go` | Diálogo de propriedades (configuração do GOROOT) |

### Alterações em arquivos existentes

| Arquivo | Descrição |
|---|---|
| `main.go` | Aplica o tema rainbow, substitui `MultiLineEntry` pelo `codeEditor`, adiciona menus de formatação, execução e propriedades |
| `terminal_panel.go` | Suporte ao diretório de trabalho do projeto aberto |

---

### Tema (`theme.go`)

- Paleta escura com fundo `#282c34`, texto `#abb2bf` e destaque primário `#61afef`
- Aplicado globalmente com `a.Settings().SetTheme(newRainbowTheme())`
- Cores de brackets definidas em `bracketPalette` (6 cores em ciclo)
- Brackets sem par correspondente usam vermelho (`bracketMismatchColor`)

### Rainbow brackets (`brackets.go`)

- Coloração de `()`, `[]` e `{}` por nível de aninhamento
- Ignora conteúdo dentro de:
  - strings (`"`, `'`, `` ` ``)
  - comentários de linha (`//`)
  - comentários de bloco (`/* */`)
- Brackets não correspondentes são marcados como erro (cor vermelha)

### Editor (`code_editor.go`)

- Substitui o `widget.MultiLineEntry` padrão do Fyne
- Renderiza código com `TextGrid` e números de linha
- Atualiza cores dos brackets em tempo real ao digitar
- Destaca a posição do cursor
- Suporta navegação (setas, Home, End), edição (Backspace, Delete, Tab, Enter) e atalhos Ctrl+C/V/X/A
- Exibe placeholder quando o editor está vazio e sem foco

### Correção de crash na inicialização

**Problema:** panic em `textgrid.go` ao abrir a IDE — o `TextGrid` com `ShowLineNumbers` era atualizado antes do layout, com largura zero, e o renderer alocava menos células do que o necessário.

**Solução:**
1. Removida a chamada a `refreshGrid()` no construtor do editor
2. Primeira renderização adiada para `CreateRenderer()`
3. Linhas montadas diretamente em `grid.Rows` (sem `SetText` + `SetStyle` individuais)
4. Largura mínima de 200px garantida antes do `Refresh` do grid

---

### Como testar

```bash
go test ./...
go build -o go-ide .
./go-ide
```

1. Abra um arquivo `.go` — funções, tipos, packages, keywords e strings devem aparecer com cores distintas
2. Verifique os rainbow brackets coloridos por nível de aninhamento
3. Verifique o tema escuro na interface (menus, explorador, editor)
4. Use **Arquivo → Formatar documento** (Shift+Alt+F) em arquivos Go
5. Use **Executar → Executar arquivo** (F5) para rodar o arquivo atual — o terminal abre automaticamente e exibe apenas a saída
6. Use **Arquivo → Propriedades** para configurar o caminho do Go
7. Abra uma pasta com código Go e teste o autocomplete: digite `fmt.` — a lista deve aparecer compacta (6 linhas) abaixo do cursor; continue digitando para filtrar; use **Enter** para aceitar ou **Esc** para fechar
