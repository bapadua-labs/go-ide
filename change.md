# Changelog

## Terminal: PTY, pasta do projeto e Ctrl+X

### Alterações

| Arquivo | Descrição |
|---|---|
| `terminal_panel.go` | `whenReady` passa a esperar o PTY real via `SetReadWriter` (não o `discardWriter` inicial); `setWorkingDir` só envia `cd` após o shell pronto; `focusActive` ao usar o painel |
| `main.go` | Ao reabrir o terminal (Ctrl+` / menu), foca o widget do terminal e força refresh do layout |
| `run.go` | `ensureTerminalOpen` foca o terminal; F5 resolve o binário `go` com `resolveGoBinary` |
| `settings.go` | `resolveGoBinary`: usa o GOROOT configurado se válido, senão o `go` do PATH |

### Problemas corrigidos

**Comandos descartados:** o terminal da lib inicia com `discardWriter`, cujo `Write` sempre “sucede”. O `whenReady` antigo considerava o shell pronto imediatamente e F5/`cd`/saída iam para o lixo até uma segunda tentativa.

**Pasta do projeto:** ao abrir outra pasta, o `cd` no shell ativo era enviado sem esperar o PTY — podia sumir e o cwd do terminal ficava divergente do root do projeto.

**Ctrl+X no terminal:** ao reabrir o painel sem criar aba nova, o foco permanecia no editor; Ctrl+X virava recortar no código em vez de ir para o shell (`0x18`).

**Instalação Go divergente:** F5 usava só `{GOROOT}/bin/go`. Com preferência/GOROOT desatualizado, a execução falhava mesmo com `go` válido no PATH.

### Solução

1. `armReady` / `SetReadWriter` sinaliza prontidão só quando o PTY real abre
2. `setWorkingDir` e `whenReady` aguardam esse sinal (até ~10 s) antes de escrever no terminal
3. `toggleTerminal`, `openTerminalTab` e `ensureTerminalOpen` focam o terminal ativo
4. `resolveGoBinary` tenta o GOROOT das Propriedades e faz fallback para `LookPath("go")`

---

## Seleção de texto no editor

### Alterações

| Arquivo | Descrição |
|---|---|
| `code_editor.go` | Estado de seleção (âncora/cursor), highlight, drag com mouse, Shift+setas, copy/cut/delete/paste sobre o intervalo |
| `code_editor_test.go` | Testes de offsets, select all, delete, insert sobre seleção e colapso/extensão com setas |

### Problema corrigido

**Seleção inexistente:** copy/cut operavam no documento inteiro (`selectionOffsets` sempre `0..len`), Ctrl+A só movia o cursor, e não havia highlight nem arraste com o mouse.

### Comportamento

- **Selecionar:** arrastar com o mouse, Shift+setas/Home/End, Shift+clique ou Ctrl+A
- **Apagar:** Backspace/Delete removem o intervalo selecionado
- **Copiar / Cortar:** Ctrl+C e Ctrl+X atuam só no trecho selecionado
- **Substituir:** digitar ou colar (Ctrl+V) substitui a seleção
- **Colapsar:** seta sem Shift fecha a seleção (esquerda → início, direita → fim)
- Highlight visual com `ColorNameSelection` do tema

---

## Ícones nos menus

### Alterações

| Arquivo | Descrição |
|---|---|
| `main.go` | Ícones do tema Fyne em todos os itens dos menus **Arquivo**, **Exibir**, **Terminal**, **Executar** e **Navegação** |

### Comportamento

- Cada item de menu exibe um ícone à esquerda do rótulo (documento, pasta, salvar, play, busca, etc.)
- Pastas recentes usam ícone de pasta
- Atalhos de teclado existentes permanecem inalterados

---

## Correção de popups LSP (hover e assinatura)

### Alterações

| Arquivo | Descrição |
|---|---|
| `lsp_util.go` | `plainTextFromMarkdown`, `wrapPlainText` e `expandSnippet` para sanitizar conteúdo LSP |
| `lsp_util_test.go` | Testes de conversão markdown, quebra de linha e expansão de snippets |
| `lsp_popup.go` | Popup com `widget.Label` e quebra de linha (substitui `canvas.Text` monolíneo) |
| `code_editor.go` | Posicionamento do hover abaixo da linha; expansão de snippets ao aceitar autocomplete |

### Problema corrigido

**Caracteres especiais sobrepondo o código:** ao passar o mouse sobre um identificador, a documentação markdown do gopls (blocos `` ``` ``, separadores `---`, etc.) era renderizada em uma única linha longa sobre o texto do editor, gerando caracteres `` e texto fantasma ilegível.

### Solução

1. Markdown convertido para texto simples antes de exibir (remove fences, separadores e formatação inline)
2. Texto quebrado em linhas com largura máxima de 480 px
3. Popup posicionado **abaixo** da linha do cursor, não sobre ela
4. Snippets LSP (`${1:nome}`, `$0`) expandidos ao aceitar autocomplete
5. Caractere `▸` substituído por `>` na ajuda de assinatura (evita glifo ausente na fonte monoespaçada)

---

## Recursos LSP do gopls

### Novos arquivos

| Arquivo | Descrição |
|---|---|
| `gopls_client.go` | Handler LSP do cliente (`PublishDiagnostics`) |
| `lsp_util.go` | Conversão de locations, hover, rename, diagnósticos e aplicação de `WorkspaceEdit` |
| `lsp_popup.go` | Popups de hover e signature help; estilo visual de diagnósticos |

### Alterações

| Arquivo | Descrição |
|---|---|
| `gopls.go` | Capabilities LSP ampliadas; métodos `definition`, `references`, `hover`, `signatureHelp`, `rename` |
| `gopls_integration.go` | Integração de navegação, referências, rename, hover, signature help e diagnósticos |
| `code_editor.go` | Sublinhado de diagnósticos, hover com mouse, Ctrl+clique, popups LSP |
| `main.go` | Menu **Navegação** e atalhos F12, Shift+F12, F2 |

### Recursos

| Recurso | Comportamento |
|---|---|
| **Ir para definição** | F12 ou Ctrl+clique — abre o arquivo e posiciona o cursor na declaração (inclui stdlib e módulos) |
| **Encontrar referências** | Shift+F12 — lista todas as ocorrências; clique navega até ela |
| **Diagnósticos** | Erros e avisos do gopls sublinhados em tempo real via `PublishDiagnostics` |
| **Renomear símbolo** | F2 — renomeia em todos os arquivos do workspace via `WorkspaceEdit` |
| **Signature help** | Ao digitar `(` — balão com assinatura e parâmetro ativo |
| **Hover** | Mouse sobre identificador — documentação do gopls (debounce 400 ms) |

### Atalhos

| Ação | Atalho |
|---|---|
| Ir para definição | **F12** ou **Ctrl+clique** |
| Encontrar referências | **Shift+F12** |
| Renomear símbolo | **F2** |

---

## Persistência de estado da IDE

### Novos arquivos

| Arquivo | Descrição |
|---|---|
| `config.go` | Carrega e salva o estado da IDE em JSON (`last_folder`, `recent_folders`) |

### Alterações

| Arquivo | Descrição |
|---|---|
| `main.go` | Reabre a última pasta ao iniciar, menu de pastas recentes, `openFolderPath` centralizado e save ao fechar |

### Arquivo de configuração

Salvo em `~/.config/go-ide/state.json`:

```json
{
  "last_folder": "/caminho/para/projeto",
  "recent_folders": [
    "/caminho/para/projeto",
    "/outro/projeto"
  ]
}
```

### Comportamento

- **Reabrir na inicialização:** se `last_folder` existir e for válida, a pasta é aberta automaticamente ao iniciar a IDE
- **Pastas recentes:** ao abrir uma pasta, ela entra na lista (máx. 10), sem duplicatas, com a mais recente primeiro
- **Menu Arquivo:** após "Abrir pasta...", um separador lista as pastas recentes (rótulo = nome da pasta)
- **Persistência:** salva ao abrir pasta e ao sair da IDE
- **Limpeza:** pastas inexistentes são removidas da lista ao carregar a configuração

---

## Atalhos de desfazer e fechar

### Alterações

| Arquivo | Descrição |
|---|---|
| `code_editor.go` | Pilha de undo (até 100 entradas), `TypedShortcut` para atalhos nativos do Fyne e callback `onAppShortcut` para atalhos globais |
| `code_editor_test.go` | Testes da pilha de undo e do atalho `ShortcutUndo` |
| `main.go` | Ctrl+W para fechar a IDE, item "Fechar" no menu Arquivo e repasse de atalhos globais quando o editor está em foco |

### Atalhos

| Ação | Comportamento |
|---|---|
| **Ctrl+Z** | Desfaz a última edição (digitação, exclusão, colar, recortar, autocomplete) |
| **Ctrl+W** | Fecha a IDE com confirmação se houver alterações não salvas |

### Problema corrigido

**Ctrl+Z não funcionava:** o Fyne intercepta Ctrl+Z como `ShortcutUndo` e envia para `TypedShortcut`, não para `TypedKey`. O editor não implementava `fyne.Shortcutable`, então o atalho era ignorado. Corrigido implementando `TypedShortcut` com suporte a Undo, Paste, Copy, Cut e SelectAll.

**Ctrl+W com editor em foco:** atalhos globais registrados no canvas não chegavam ao handler quando o editor tinha foco (widget `Shortcutable` consome o evento). Corrigido repassando atalhos não tratados via `onAppShortcut`.

---

## Cursor do editor

### Alterações

| Arquivo | Descrição |
|---|---|
| `code_editor.go` | Caret vertical piscante, posicionamento por clique do mouse e alinhamento via API do `TextGrid` |

### Comportamento

- **Caret visível:** barra vertical de 2px na cor do texto, piscando a cada 530 ms quando o editor tem foco
- **Clique do mouse:** reposiciona o cursor na linha e coluna correspondentes ao ponto clicado
- **Reset do piscar:** ao digitar, mover o cursor ou clicar, o caret volta a ficar visível
- **Scroll:** o caret acompanha o deslocamento do conteúdo ao rolar o editor

### Problemas corrigidos

**Cursor pouco visível:** o destaque de fundo na célula do `TextGrid` foi substituído por um caret dedicado, mais perceptível e com animação de piscar.

**Clique não movia o cursor:** `Tapped` apenas focava o editor; agora converte a posição do clique em linha/coluna, considerando gutter de numeração, scroll e UTF-8.

**Caret desalinhado (1–2 colunas à frente):** o posicionamento manual usava tamanho de célula sem arredondamento e conflitava com o scroll interno do `TextGrid`. Corrigido desativando o scroll interno (`ScrollNone`) e usando `PositionForCursorLocation` / `CursorLocationForPosition` do próprio grid.

---

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

> **Nota:** o critério de prontidão do item 2 ainda falhava com o `discardWriter` da lib; ver a entrada **Terminal: PTY, pasta do projeto e Ctrl+X**.

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
- Destaca a posição do cursor com caret vertical piscante
- Suporta navegação (setas, Home, End), edição (Backspace, Delete, Tab, Enter) e atalhos Ctrl+C/V/X/A/Z
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
8. Clique em qualquer ponto do código — o cursor deve ir para a posição clicada; verifique o caret piscando alinhado ao texto
9. Digite no editor e use **Ctrl+Z** para desfazer a última ação
10. Use **Ctrl+W** (ou **Arquivo → Fechar**) para fechar a IDE
11. Feche a IDE com uma pasta aberta e reinicie — a última pasta deve reabrir automaticamente
12. Abra pastas diferentes e verifique em **Arquivo** a lista de pastas recentes abaixo de "Abrir pasta..."
13. Com um `.go` aberto e pasta de projeto: **F12** ou **Ctrl+clique** em um símbolo — deve ir à definição
14. **Shift+F12** em um identificador — lista referências; clique em uma linha para navegar
15. Introduza um erro de sintaxe — o trecho deve aparecer sublinhado; passe o mouse para ver a mensagem
16. **F2** para renomear uma função/struct — confirme que todas as ocorrências no projeto foram atualizadas
17. Digite `fmt.Println(` — deve aparecer ajuda de assinatura abaixo do cursor
18. Passe o mouse sobre `fmt` ou outro identificador — documentação em popup legível, abaixo da linha, sem sobrepor o código
19. Verifique os ícones nos menus **Arquivo**, **Navegação**, **Exibir**, **Terminal** e **Executar**
20. Aceite um autocomplete com snippet (ex.: `func`) — placeholders `${1:...}` e `$0` não devem aparecer no código
