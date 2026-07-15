# lastfm-miner

O lastfminer é um daemon criado para minerar as recomendações do Last.fm. O objetivo é automatizar o download sob demanda de faixas avulsas diretamente para a biblioteca de servidores de música como Gonic ou Navidrome, contornando a limitação conceitual de ferramentas como o Lidarr, que gerenciam apenas discografias completas.

O sistema utiliza processamento assíncrono baseado em canais nativos do Go (Fila FIFO), notificações em tempo real via Server-Sent Events (SSE) e enriquecimento de metadados através da API do iTunes.

## Funcionalidades

- Extração de Recomendações Reais: Consome o endpoint interno do Last.fm, obtendo o algoritmo exato da página "All Recommendations", sem a necessidade de chaves de API.
- Enriquecimento de Metadados via iTunes API: Consulta os metadados da Apple para identificar o título oficial da faixa, nome do álbum, ano de lançamento e número da faixa.
- Capas em Alta Resolução: Faz o download da arte oficial do álbum (cover.jpg) diretamente para o diretório do álbum.
- Padronização de Arquivos (Scene Standard): Organiza a biblioteca de mídia de forma física e lógica no disco utilizando a taxonomia: Artista / Album (Ano) / Artista - Album - Track - Nome.mp3.
- Injeção de ID3 Tags Completa: Grava os metadados extraídos e incorpora a capa de alta resolução diretamente no header do MP3 via mid3v2.
- Interface Baseada em HTMX e SSE: Single-Page Application (SPA) nativa. Logs em tempo real via Server-Sent Events.
- Arquitetura de Fila FIFO: Downloads processados sequencialmente através de Go Channels.
- Persistência simples: Armazenamento de dados locais e limites de downloads persistidos em um arquivo JSON simples.
- Tarefa agendada: Goroutine dedicada que acorda a cada 7 dias para varrer o perfil do usuario e injetar novas descobertas na fila de processamento de forma totalmente silenciosa.

## Estrutura de Diretórios Gerada

O comportamento de escrita no disco rígido replica fielmente a organização esperada por players avançados (como o Symfonium) e scanners rigorosos baseados em caminhos físicos (como o Gonic):

```text
/musics/
└── Within Thy Wounds/
    └── Forest of Iniquity (2023)/
        ├── cover.jpg
        └── Within Thy Wounds - Forest of Iniquity - 03 - Font of All Holiness.mp3

```

## Arquitetura de Execução

O ciclo de vida da aplicação divide-se em três camadas isoladas operando em concorrência segura:

1. Camada Web (HTTP Server): Escuta na porta 8080 servindo os templates HTML estáticos e respondendo a chamadas parciais acionadas por atributos HTMX.
2. Camada de Sincronização (Scraper & API Client): Interage com as requisições HTTP externas para o Last.fm e a API do iTunes, decodificando as cargas JSON para estruturas em memória.
3. Camada de Download (Worker Queue): Um loop de execução alimentado por um canal thread-safe. Invoca os utilitários yt-dlp, ffmpeg e mid3v2 de forma encadeada.


## Instalação via Docker (Recomendado)

A aplicação foi projetada para rodar de forma isolada em um contêiner Docker, empacotando automaticamente todas as dependências necessarias.

## Dependências
 * Git
 * Docker (plugin docker compose)

Execute os seguintes comandos:

```bash
# 1. Clone o repositório
git clone https://github.com/omifares/lastfminer.git

# 2. Acesse a pasta do projeto
cd lastfminer

# 3. Construa a imagem local e suba o contêiner em segundo plano
docker compose up -d --build
```


## Dependencias (Bare-metal)

Caso opte por executar o binário diretamente no sistema host, certifique-se de possuir as seguintes dependências instaladas e visíveis no PATH global do sistema:

* Go (versão 1.20 ou superior)
* yt-dlp (versão atualizada para compatibilidade com assinaturas SABR do YouTube)
* FFmpeg e FFprobe (para extração do fluxo de áudio e conversão em MP3)
* Python 3 (para uso do yt-dlp)
* py-mutage (responsável por disponibilizar o mid3v2)
* Node.js (versão igual ou superior a 22.0) ou Deno (necessário como runtime JS do yt-dlp para solucionar desafios de assinatura do YouTube)

## Instalação Nativa (Bare-Metal)

Se preferir rodar fora de contêineres, siga os passos abaixo no terminal:

1. Inicialize o módulo e sincronize as dependências da biblioteca padrão do Go:

```bash
go mod init github.com/omifares/lastfminer
go mod tidy

```

2. Compile o binário otimizado para produção removendo os símbolos de depuração para reduzir o tamanho do executável:

```bash
go build -ldflags="-w -s" -o lastfminer main.go

```

3. Inicie o servidor executando o binário gerado:

```bash
./lastfminer

```

## Configuração e Uso

1. Abra o navegador e acesse a interface através do endereço `http://localhost:8080`.
2. No painel esquerdo "Configuração Base", insira o seu nome de usuario público do Last.fm e o limite máximo de faixas que deseja processar a cada ciclo de sincronização (tracksDownloadLimit). Clique em "Salvar & Persistir".
3. No painel direito, clique no botão "Sincronizar" para forçar uma chamada imediata à rádio de recomendações. A lista será preenchida automaticamente.
4. Utilize o botão "Por na Fila" em faixas individuais ao passar o mouse sobre ela, ou utilize o botão "Baixar Todas" no cabeçalho para enfileirar em massa todas das faixas recomendadas.
5. Monitore o progresso do download no componente "Registo de Transferências" em tempo real.


## Disclaimer / Aviso Legal

> Este projeto é estritamente uma ferramenta de automação residencial, agregação de metadados e estudo conceitual de desenvolvimento de software em Go.
>
> 1. Sem Hospedagem de Conteúdo: O lastfm-miner não hospeda, não armazena, não distribui e não mantém nenhuma obra musical, arquivo de mídia ou qualquer conteúdo protegido por direitos autorais em seu código-fonte, servidores ou repositórios.
>
> 2. Mecanismo de Funcionamento: A aplicação limita-se a interagir com endpoints públicos e APIs de dados (iTunes e Last.fm) para estruturar informações de catalogação. O download físico de fluxos de áudio e a conversão de formatos são delegados inteiramente a utilitários de terceiros (como o yt-dlp e FFmpeg), cuja instalação, execução e manutenção dependem exclusivamente do ambiente configurado pelo usuário.
>
> 3. Responsabilidade do Usuário: Os desenvolvedores e colaboradores deste projeto não assumem qualquer responsabilidade pelo uso da ferramenta ou por eventuais violações de termos de serviço de plataformas de terceiros. É de responsabilidade exclusiva do usuário final garantir que a utilização desta automação e o armazenamento dos arquivos gerados estejam em estrita conformidade com as leis de direitos autorais locais e as legislações vigentes de sua jurisdição.
