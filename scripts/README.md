# VPS batch: top-50 за 2 года

Скрипт [`batch_top50_vps.sh`](batch_top50_vps.sh) по одной монете из [`symbols_top50.txt`](symbols_top50.txt)
(стейблы и wrapped в список не входят; скрипт дополнительно пропускает известные stable bases):

1. `fetch-data` — 1m CSV за последние 2 года  
2. `scan-markets` — все алгоритмы, 3 периода, JSON + OHLC  
3. удаляет CSV (результаты остаются в `reports/`)  
4. в конце — `report-html` и `reports/batch_results_YYYYMMDD.tar.gz`

Resume: перезапуск пропускает символы из `reports/batch_state/done.txt`.

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
ALGOS=liquidity-sweep-long-v2,nr7-trend-breakout-v1 \
  SYMBOLS_FILE=scripts/symbols_top50.txt \
  ./scripts/batch_top50_vps.sh
```

## Артефакты

```
reports/
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
- На 10% CPU полный grid × 50 монет может идти **сутки+**.
