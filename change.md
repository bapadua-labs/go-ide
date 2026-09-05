# Changelog

## Correção da execução no terminal

### Alterações

| Arquivo | Descrição |
|---|---|
| `run.go` | Execução via `exec.Command` em vez de comando no shell; `ensureTerminalOpen` sincroniza o painel antes de rodar |
| `terminal_panel.go` | `whenReady` aguarda o shell ficar pronto; `runGoFile` e `displayOutput` exibem só a saída do programa |

### Problemas corrigidos

**Execução exigia dois cliques:** ao pressionar F5 com o terminal fechado, o painel abria mas o comando era enviado antes do shell estar pronto (layout e PTY ainda não inicializados). O comando falhava silenciosamente e era preciso executar de novo.

**Comando visível no terminal:** o `go run` era digitado no shell interativo e aparecia ecoado junto com a saída.

### Solução

1. `ensureTerminalOpen` abre o painel, cria a aba se necessário e força refresh do layout
2. `whenReady` aguarda o terminal aceitar entrada (até ~10 s) antes de executar
3. `go run` é executado diretamente com `exec.Command`, capturando stdout e stderr
4. Apenas o resultado é exibido no terminal (com `stty -echo` + `base64` no Linux para ocultar o script intermediário)

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

1. Abra um arquivo `.go` — os brackets devem aparecer coloridos por nível
2. Verifique o tema escuro na interface (menus, explorador, editor)
3. Use **Arquivo → Formatar documento** (Shift+Alt+F) em arquivos Go
4. Use **Executar → Executar arquivo** (F5) para rodar o arquivo atual — o terminal abre automaticamente e exibe apenas a saída
5. Use **Arquivo → Propriedades** para configurar o caminho do Go
