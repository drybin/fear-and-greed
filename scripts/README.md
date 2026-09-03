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

Для отдельной проверки новой RSI trend mean-reversion гипотезы используйте
изолированный suite `rsi-mean-reversion-v1`. Он не меняет старый
`mean-reversion-v1`: вход требует, чтобы RSI(14) вернулся выше 35 после
перепроданности, при этом полностью закрытый 4h бар подтверждает растущую
EMA200; цель -- 1h EMA20, тайм-аут -- 48 часов.

```bash
SUITE=rsi-mean-reversion-v1 ./scripts/run_research_v2.sh
```

Для независимой проверки trend-continuation гипотезы используйте
`donchian-breakout-v1`: 4h close должен пересечь максимум предшествующего
Donchian-канала в растущем EMA200-тренде. Сетка фиксирует только длину канала
(`20`/`40`) и начальный ATR-stop (`1.5`/`2.0`); выходы заранее заданы как 1R
частично, 3R полностью или через 21 день. Трейлинг не имитируется, поскольку
текущий protocol-v2 не поддерживает динамический stop без изменения движка.

```bash
SUITE=donchian-breakout-v1 ./scripts/run_research_v2.sh
```

Для проверки независимой range-reversion гипотезы используйте
`bollinger-range-reversion-v1`: на 1h цена должна закрыться ниже нижней
Bollinger-полосы, а затем закрыться обратно внутри неё при ADX(14) не выше
`20` или `25`. Сетка фиксирует порог ADX и ширину полос (`2.0`/`2.5σ`); стоп
ниже импульсного минимума или на 1.5 ATR, выходы -- средняя и верхняя полосы
либо 48 часов.

```bash
SUITE=bollinger-range-reversion-v1 ./scripts/run_research_v2.sh
```

Ресурсоёмкие фазы `development`, `freeze`, `review` и `final` по умолчанию
ограничены `GOMEMLIMIT=512MiB GOGC=20`, чтобы длинный прогон не был остановлен
VPS по памяти. При достаточном объёме RAM значения можно явно переопределить
через одноимённые переменные окружения.

## Portfolio: ablation режимов рынка

После первого диагностического результата relative strength сравнивается без
изменения ranking, риска или данных. Скрипт последовательно создаёт четыре
отдельных immutable запуска: `both`, `btc-ema`, `breadth` и `none`.

```bash
RESEARCH_MANIFEST=/home/drybin/fear-and-greed/data/research-v2/runs/2026-08-01-0de0bb138075/manifest.json \
  ./scripts/run_portfolio_regime_ablation.sh
```

Сначала требуется чистый закоммиченный worktree и свежий CLI:

```bash
make build-cli
```

CSV повторно не загружаются. Скрипт запускает проверку Go-тестов в Docker и
сверяет fingerprint каждого входного CSV перед каждым вариантом. По умолчанию
отчёты будут лежать в `data/research-v2/portfolio-runs/regime-ablation-<git-sha>/`.

## Portfolio: breadth pullback

Следующий независимый кандидат сохраняет лучший защитный режим `breadth`, но
меняет сам вход: лидер cross-sectional ranking покупается только когда его
последняя завершённая дневная цена выше EMA20 и находится не дальше `0.5 ATR`
от неё. Открытие следующего дня выше этого frozen лимита отклоняется как
`entry_extension`.

```bash
RESEARCH_MANIFEST=/home/drybin/fear-and-greed/data/research-v2/runs/2026-08-01-0de0bb138075/manifest.json \
  ./scripts/run_portfolio_breadth_pullback.sh
```

Результат сохраняется отдельно в
`data/research-v2/portfolio-runs/breadth-pullback-<git-sha>/`; текущий
baseline и ablation не перезаписываются.

## Portfolio: breadth pullback walk-forward

После одного диагностического прогона параметры кандидата больше не меняются.
Этот сценарий запускает ровно тот же `breadth + trend-pullback` на пяти
непересекающихся development-окнах и не открывает locked holdout
`2026-05-03`--`2026-08-01`:

```bash
RESEARCH_MANIFEST=/home/drybin/fear-and-greed/data/research-v2/runs/2026-08-01-0de0bb138075/manifest.json \
  ./scripts/run_portfolio_breadth_pullback_walk_forward.sh
```

Сначала нужен чистый закоммиченный worktree и свежий CLI (`make build-cli`).
Каждое окно получает собственные immutable manifest и report, а краткая
машиночитаемая сводка записывается в
`data/research-v2/portfolio-runs/breadth-pullback-walk-forward-<git-sha>/summary.json`.
CLI фиксирует границы в manifest через `--start YYYY-MM-DD --end YYYY-MM-DD`;
он отклоняет диапазон за пределами development-горизонта исходного исследования.

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
