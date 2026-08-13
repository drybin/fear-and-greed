# VPS batch: CMC universe за 2 года

## Protocol-v2: полный воспроизводимый прогон

После фиксации точной версии кода в чистом Git-коммите запустите:

```bash
./scripts/run_research_v2.sh
```

Скрипт последовательно выполняет `fetch-data`, `verify`, `prepare`,
`development`, `freeze` и компактный pre-holdout `review`. Повторный запуск продолжает checksum-valid
checkpoints. Данные, manifest, журнал и результаты сохраняются под
`data/research-v2/`; CSV не удаляются. Скрипт ничего не компилирует: готовый
CLI должен находиться в `bin/cli`, либо его путь передаётся через
`BIN`.

Для следующего независимого исследования используйте suite из двух новых
кандидатов, не смешивая его с отвергнутым core-v2 запуском:

```bash
SUITE=research-v3 ./scripts/run_research_v2.sh
```

Это создаст новый manifest и experiment ID. Relative strength намеренно не
входит в этот прогон: он требует следующего portfolio-исследования.

Незавершённые run-директории старых revisions можно сначала посмотреть, а
затем явно удалить:

```bash
./scripts/cleanup_research_v2_runs.sh
./scripts/cleanup_research_v2_runs.sh --apply
```

Frozen/final и текущий revision сохраняются. Для удаления также текущего
незавершённого run нужен отдельный флаг `--include-current`.

Для `verify` используется Go на хосте. Если Go отсутствует, скрипт
автоматически запускает тесты в `golang:1.22` через Docker, не пересобирая и не
заменяя `bin/cli`. Если точный коммит уже был проверен вручную при сборке,
проверку можно явно пропустить:

```bash
SKIP_VERIFY=1 ./scripts/run_research_v2.sh
```

По умолчанию скрипт останавливается до одноразового holdout. После проверки
freeze явно авторизуйте final:

```bash
AUTHORIZE_HOLDOUT=1 SKIP_FETCH=1 ./scripts/run_research_v2.sh
```

Если development полностью завершился, но `freeze` был прерван из-за ресурсов,
его можно заморозить исправленным CLI без повторного расчёта 177 unit-ов:

```bash
./bin/cli research-validate freeze --existing-development \
  --manifest /absolute/path/to/existing/manifest.json \
  --candle-dir /absolute/path/to/data/research-v2 \
  --output /absolute/path/to/existing/output \
  --workdir /absolute/path/to/repository
```

Этот режим не запускает evaluator и сохраняет исходный source hash готового
development.

Для уже замороженного запуска сначала сформируйте review, не открывая holdout:

```bash
FINAL_ONLY=1 RUN_DIR=/absolute/path/to/run ./scripts/run_research_v2.sh
```

Если бинарник содержит только проверенные исправления orchestration поверх
frozen commit, final разрешается отдельным флагом. Git автоматически отклонит
любое изменение стратегий, execution, metrics или загрузки данных:

```bash
FINAL_ONLY=1 AUTHORIZE_HOLDOUT=1 ORCHESTRATION_UPGRADE=1 \
  RUN_DIR=/absolute/path/to/run ./scripts/run_research_v2.sh
```

После прерывания эта же команда продолжает недостающие holdout units под
неизменным `opened.json`; уже завершённые units повторно не рассчитываются.

Период можно зафиксировать явно:

```bash
CUTOFF=2026-08-01 SINCE=2024-07-01 UNTIL=2026-07-31 \
  ./scripts/run_research_v2.sh
```

`CUTOFF` является исключительной UTC-границей, а `UNTIL` — последним
включённым календарным днём Binance-загрузки.

Скрипт [`batch_top50_vps.sh`](batch_top50_vps.sh) гоняет монеты из файла символов
(стейблы и wrapped в списки не входят; скрипт дополнительно пропускает известные stable bases):

1. `fetch-data` — 1m CSV за последние 2 года  
2. `scan-markets` — все алгоритмы, 3 периода, JSON + OHLC  
3. удаляет CSV (результаты остаются в `reports/`)  
4. в конце — `report-html` и `reports/batch_results_YYYYMMDD.tar.gz`

Resume: перезапуск пропускает символы из `$REPORT_DIR/batch_state/done.txt`.

## Списки символов

| Файл | Смысл |
|------|--------|
| [`symbols_top50.txt`](symbols_top50.txt) | CMC ~top-50 → Binance spot USDT (50 пар) |
| [`symbols_cmc50_200.txt`](symbols_cmc50_200.txt) | CMC #50–200 → Binance spot USDT, **без** пересечения с top-50 (**73** пары) |

## Сборка на Linux VPS (amd64)

```bash
# на машине разработки (кросс-сборка) или на VPS с Go:
GOOS=linux GOARCH=amd64 go build -o bin/cli ./cmd/cli/...

chmod +x scripts/batch_top50_vps.sh
./scripts/batch_top50_vps.sh
```

Лучше запускать под `tmux`/`screen` (сессия виртуальной консоли может оборваться):

```bash
tmux new -s batch
./scripts/batch_top50_vps.sh
# Detach: Ctrl-b d
```

### Прогон CMC #50–200 (отдельно от top-50)

Используйте **отдельный `REPORT_DIR`**, чтобы не смешать `done.txt` и JSON с первым батчем:

```bash
tmux new -s batch50_200
REPORT_DIR=./reports_cmc50_200 \
  SYMBOLS_FILE=scripts/symbols_cmc50_200.txt \
  ./scripts/batch_top50_vps.sh
```

Ориентир: ~1.5–2 суток на 10% CPU, ~1.5–2 GB под JSON/OHLC; свободно лучше ≥10 GB.

## Переменные окружения

| ENV | Default | Смысл |
|-----|---------|--------|
| `BIN` | `./bin/cli` | путь к CLI |
| `DATA_DIR` | `./data` | CSV (временно) |
| `REPORT_DIR` | `./reports` | артефакты |
| `SYMBOLS_FILE` | `scripts/symbols_top50.txt` | список пар |
| `SINCE` | today−2y UTC | начало истории |
| `LAST_YEARS` | `2` | и для `--last-years`, и для default `SINCE` |
| `ALGOS` | полный список (см. скрипт) | CSV id алгоритмов |
| `MARKET` | `spot` | `spot` / `futures` |
| `GOMAXPROCS` | `1` | лимит CPU Go |
| `NICE_LEVEL` | `10` | `nice -n` |
| `MIN_FREE_GB` | `2` | abort если мало места на диске |

Пример урезанного прогона:

```bash
ALGOS=nr7-trend-breakout-v1,fib-pullback-trend-v1,volatility-compression-breakout-v1,breakout-retest-long-v2 \
  REPORT_DIR=./reports_cmc50_200 \
  SYMBOLS_FILE=scripts/symbols_cmc50_200.txt \
  ./scripts/batch_top50_vps.sh
```

## Артефакты

```
reports/   # или reports_cmc50_200/
  data/<algo>/SYMBOL__PERIOD.json
  data/<algo>/SYMBOL__PERIOD__ohlc_240m.json
  report.html          # после финального report-html
  batch_state/
    done.txt
    failed.txt
    batch_run.log
    current.log
  batch_results_YYYYMMDD.tar.gz
```

Выгрузка с VPS без внешнего IP — отдельно (архив уже лежит на диске).

## Важно

- При `--symbol` `scan-markets` чистит **только файлы этой монеты** в `reports/data/<algo>/`, остальные символы не трогает.
- Список `ALGOS` в скрипте нужно синхронизировать с Usage `--algo` в CLI, если добавляете новые стратегии.
- На 10% CPU полный grid × 50 монет может идти **сутки+**; ×73 (CMC 50–200) — ориентир **1.5–2 суток**.
- Часть CMC #50–200 нет на Binance spot (PI, KAS, FLR…) — в файл не попали; fetch по ним всё равно бы падал.
