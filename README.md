# layout-tray (Go)

Ícone na bandeja do Windows que mostra o layout de teclado ativo:
bandeira do Brasil para **Português (ABNT2)** e dos EUA para **US-International**.
Clique direito no ícone → **Sair**.

Porte do projeto em Rust. Só a biblioteca padrão do Go (sem módulos externos,
sem windres).

## Compilar

    build.cmd

Gera `layout-tray.exe` (sem janela de console, com ícone e versão embutidos).
O `.syso` com o ícone já está no projeto; `go build` sozinho também o inclui.
Para trocar o ícone, substitua `app.ico` e rode `build.cmd` (ele usa
`tools/mkrsrc`, escrito em Go).

## Como funciona

- A cada 500 ms lê o layout da thread da janela em primeiro plano
  (`GetKeyboardLayout`); a palavra alta do HKL decide: `0xF010` = PT-BR,
  `0xF001` = US-International. Qualquer outro layout mantém a bandeira atual.
- As bandeiras (`assets/*.png`) vão embutidas no executável e são redimensionadas
  para o tamanho do ícone de bandeja do DPI atual.
- Menu, ícone com alfa, reinício do Explorer (`TaskbarCreated`) e DPI vêm do
  código do mouse-battery.

## Diferenças em relação ao Rust

- O ícone inicial já reflete o layout em uso (no Rust ele começava sempre com a
  bandeira dos EUA até o layout mudar).
- Tooltip único: "US International" (o Rust usava "US-International" no início).
- Só uma instância por vez.
- Timer do Windows em vez de `sleep` + laço de mensagens.
