package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Ícones outline 16×16 no estilo da toolbar do explorador do VS Code / Cursor.
var (
	explorerNewFileIcon = theme.NewThemedResource(fyne.NewStaticResource(
		"explorer-new-file.svg",
		[]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16">
<path fill="currentColor" fill-rule="evenodd" d="M4 1.5h5.5l.35.15L13 4.85V6.5h-1V5H9V2.5H4v11h5v1H3.5l-.5-.5v-12l.5-.5zm6 .7V4h2.2L10 2.2zM12.5 9H11v2H9v1.5h2V15h1.5v-2.5H15V11h-2.5V9z"/>
</svg>`),
	))
	explorerNewFolderIcon = theme.NewThemedResource(fyne.NewStaticResource(
		"explorer-new-folder.svg",
		[]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16">
<path fill="currentColor" fill-rule="evenodd" d="M1.5 2.5v10l.5.5h12l.5-.5v-8l-.5-.5H8.2L6.9 2.65 6.5 2.5h-4.5l-.5.5zm1 .5h3.7l1.15 1.15.35.15H13.5v7.2H2.5V3zm10 5H11v2H9v1.5h2V14h1.5v-2.5H15V10h-2.5V8z"/>
</svg>`),
	))
)
