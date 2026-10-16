# Models locals: Ollama, llama.cpp i LM Studio

*[English](local-models.md)*

En un servidor local, el **prefill** (llegir el prompt) és el que més temps
triga. Un agent torna a enviar tota la conversa a cada pas. Si la caché de
KV del servidor conserva el prefix, cada pas només processa els tokens nous.
Si no hi encerta, una conversa de 30k tokens pot trigar desenes de segons en
un portàtil abans del primer token. Per això la taxa d'encerts de la caché
pesa més que gairebé qualsevol altra cosa. Hi ha dues fallades silencioses:

- **Ollama trunca el context.** El seu `num_ctx` per defecte és de 2048-4096
  tokens. Un prompt més llarg en perd el principi, és a dir, el system prompt
  i les definicions d'eina, i Ollama no dona cap error.
- **llama.cpp desplaça el context** si té `--context-shift` actiu. Quan la
  finestra s'omple, descarta tokens vells, també sense cap error.

Gregal s'ocupa de la banda del client en tots dos casos. La del servidor es
pot comprovar amb `gregal doctor`.

## Ollama: `type: ollama`

```yaml
providers:
  ollama:
    type: ollama
    base_url: http://localhost:11434/v1   # amb /v1 o sense, tots dos van bé
    api_key: ""
    # num_ctx: 65536     # opcional; vegeu més avall
    # keep_alive: 30m    # opcional; "-1" el deixa carregat per sempre
```

Amb `type: ollama`, Gregal fa servir l'API nativa `/api/chat`, amb
streaming, eines, imatges i pensament. A més:

- **Envia un `options.num_ctx` fix.** El valor es tria una sola vegada per
  model i procés, i no canvia mai d'una petició a l'altra, perquè un
  `num_ctx` diferent fa que Ollama torni a carregar el model. Gregal agafa el
  primer d'aquests que s'apliqui:
  1. el `num_ctx` del config del provider;
  2. el context amb què el model ja està carregat (`/api/ps`), si és de 16k
     o més, perquè la primera crida no el torni a carregar;
  3. 32k, o el màxim del model (`/api/show`) si és més petit.
  
  Ollama recomana 64k o més per a agents. Posa `num_ctx: 65536` si el model
  i la memòria t'ho permeten.
- **Envia `keep_alive: 30m`.** Amb els 5 minuts per defecte d'Ollama, el
  model es descarrega entre torns i es perd la caché.
- **Comprova les capacitats amb `/api/show`.** Un model sense `tools` dona
  un error clar en lloc d'una petició fallida. `think` només s'envia als
  models amb `thinking`. gpt-oss rep `low`/`medium`/`high`.
- **Fa servir aquest `num_ctx` com a finestra de context**, perquè la
  compactació comenci abans que Ollama truncaria.

Ollama no declara els tokens de la caché. Quan `prompt_eval_count` queda
clarament per sota de la mida del prompt, la diferència compta com a caché
(és una estimació). `load_duration` ha de ser gairebé 0 a partir del segon
torn. Si no ho és, el model s'està tornant a carregar.

Entorn del servidor (per a `ollama serve`):

| Variable | Valor | Per què |
|---|---|---|
| `OLLAMA_FLASH_ATTENTION` | `1` | Cal per quantitzar la caché de KV; prefill més ràpid |
| `OLLAMA_KV_CACHE_TYPE` | `q8_0` | La meitat de memòria de KV, sense pèrdua de qualitat apreciable |
| `OLLAMA_CONTEXT_LENGTH` | `32768` o més | Per defecte per als clients que no envien `num_ctx` |
| `OLLAMA_KEEP_ALIVE` | `30m` o `-1` | Per defecte per als clients que no envien `keep_alive` |

Fonts: [context length](https://github.com/ollama/ollama/blob/main/docs/context-length.mdx),
[FAQ](https://github.com/ollama/ollama/blob/main/docs/faq.mdx),
[API](https://github.com/ollama/ollama/blob/main/docs/api.md),
[recàrrega si canvia num_ctx](https://github.com/ollama/ollama/issues/9749).

## llama.cpp: `type: llamacpp`

```yaml
providers:
  llamacpp:
    type: llamacpp
    base_url: http://localhost:8080/v1
    api_key: ""
    # slot: 0          # id_slot de la sessió (-1 = que triï el servidor)
    # side_slot: 3     # id_slot de les crides laterals (per defecte, l'últim)
```

Amb `type: llamacpp`, Gregal continua amb Chat Completions i hi afegeix:

- **`cache_prompt: true`** a cada petició.
- **Un `id_slot` fix** quan el servidor té més d'un slot (`--parallel N`,
  llegit de `/props`). La sessió fa servir el slot 0. Les crides laterals
  (crític del pla, revisió automàtica, missatges de commit) fan servir
  l'últim, perquè no expulsin el prefix de la sessió. Si el slot ja té una
  petició en vol d'aquest mateix procés (subagents en paral·lel), la petició
  va sense `id_slot` i el servidor tria un slot lliure. Si no, faria cua.
- **Llegeix els `timings`.** `cache_n` passa a `cached_tokens` al comptador
  de consum. `prompt_n` i `prompt_per_second` donen la velocitat real de
  prefill.
- **Llegeix la finestra de `/props`** (`n_ctx` per slot). L'endpoint
  `/v1/models` només declara el context d'entrenament, que sol ser molt més
  gran que el que el servidor ha reservat.

Flags recomanats de `llama-server`:

| Flag | Per què |
|---|---|
| `--jinja` (per defecte a les versions actuals) | Crides d'eina natives amb la plantilla del model. Sense, o amb una plantilla que llama.cpp no reconeix, les eines cauen al format **Generic**: menys fiable i més tokens |
| `--cache-reuse 256` | Reaprofita trossos de la caché després d'un canvi enmig del prompt ([PR #9866](https://github.com/ggml-org/llama.cpp/pull/9866)) |
| `-ctk q8_0 -ctv q8_0` | La meitat de memòria de KV. q8_0 a la K fa variar la perplexitat un ~0,03%; **q4_0 a la K, un ~3,3%**, i empitjora les crides d'eina ([PR #7412](https://github.com/ggml-org/llama.cpp/pull/7412)) |
| `-fa on` | Flash attention, cal per quantitzar la V |
| `-ub 1024` (fins a 2048) | Lot físic més gran, prefill més ràpid |
| `--spec-type ngram-mod` | Descodificació especulativa amb n-grames del prompt (*prompt lookup*), sense model esborrany. Les edicions copien molt de l'entrada: entre 2 i 4 vegades més ràpid ([prompt lookup](https://github.com/apoorvumang/prompt-lookup-decoding)) |
| `-md esborrany.gguf --spec-draft-n-max 16` | Especulació amb model esborrany (els flags antics `--draft-max`/`--draft-min` ja no existeixen) |
| `--no-context-shift` | Que no descarti mai tokens vells en silenci; Gregal compacta abans que la finestra s'ompli |
| `--parallel 2` (o més) | Un slot per a la sessió i un per a les crides laterals |
| `--slot-save-path DIR` | Permet desar i restaurar un prefix escalfat |

Els models híbrids o amb finestra lliscant (Gemma 3/4, Qwen3-Next)
reaprofiten pitjor la caché. Vegeu l'[issue #19794](https://github.com/ggml-org/llama.cpp/issues/19794).

Fonts: [README del servidor](https://github.com/ggml-org/llama.cpp/blob/master/tools/server/README.md),
[function calling](https://github.com/ggml-org/llama.cpp/blob/master/docs/function-calling.md).

## LM Studio

LM Studio fa servir Chat Completions (sense `type`). A l'arrencada, Gregal
llegeix `/api/v0/models`. Si el model està carregat, la finestra és
`loaded_context_length`. Si no, és `max_context_length`. Puja el TTL
d'inactivitat de LM Studio perquè el model no es descarregui entre torns.

## vLLM i altres

vLLM declara `max_model_len` a `/v1/models`, i Gregal ja el llegeix. La
caché de prefix és activa per defecte. Per a les crides d'eina cal
`--enable-auto-tool-choice --tool-call-parser <família>`.

## `gregal doctor`

Per a cada provider local que fa servir algun rol, `gregal doctor` mostra:

- **llama.cpp**: `n_ctx` i slots, amb un avís si `--context-shift` és
  actiu, si la plantilla no sap pintar eines (format Generic), si el context
  és de menys de 16k o si al provider li falta `type: llamacpp`. Després, una
  línia de suggeriments amb els flags que no pot confirmar
  (`--cache-reuse 256`, `-ctk q8_0 -ctv q8_0`, `-fa on`, `-ub 1024`,
  `--spec-type ngram-mod`).
- **Ollama**: el `num_ctx` que Gregal enviarà i d'on surt, amb un avís si és
  de menys de 16k, si el model no té la capacitat `tools` o si el provider
  encara fa servir `/v1` sense `type: ollama`.
- **LM Studio**: el context del model de cada rol i si està carregat.

Els avisos (`AVÍS`) i els suggeriments (`INFO`) no fan fallar el `doctor`.
