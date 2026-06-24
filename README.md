# fear-and-greed

Исследование торговых стратегий для криптовалют (BTC, ETH и др.) с опорой на индекс **Crypto Fear & Greed** и рыночные данные Binance.

Проект собирает историю цен и индекса, затем бэктестит простые правила входа/выхода. Это **исследовательский CLI**, а не продакшен-бот.

Старт проекта: 27.06.2025

## Возможности

| Компонент | Описание |
|-----------|----------|
| **`fetch-data`** (Go) | OHLCV с Binance Spot или USD-M Futures → CSV в `data/` |
| **`fear8.php` / `fear8_hour.php`** | Fear & Greed (alternative.me) + цены Binance → CSV в корне (legacy) |
| **`fear-research`** (Go) | Бэктест на legacy CSV в корне (не меняем) |
| **`scan-markets`** (Go) | Random buy / sell +N% по `data/*.csv`, sweep N=1–100%, 3 периода |
| **`pkg/telegram`** | Клиент Telegram API (пока не подключён к usecase) |

## Быстрый старт

```bash
go mod download

# опционально: .env в корне (APP_NAME, TG_BOT_TOKEN, TG_CHAT_ID)

go run ./cmd/cli/... hello-world

# загрузка свечей (рекомендуемый способ)
go run ./cmd/cli/... fetch-data --symbol ETHUSDT --interval 1h --since 2024-01-01

# бэктест (читает legacy CSV, см. раздел «Данные»)
go run ./cmd/cli/... fear-research
```

Сборка бинарника:

```bash
go build -o bin/fear-and-greed ./cmd/cli/
./bin/fear-and-greed fetch-data --help
```

Тесты:

```bash
make unit-test
# или
go test ./...
```

## Команды CLI

### `fetch-data`

Скачивает историю свечей с Binance и сохраняет в каталог `data/` (каталог в `.gitignore`).

| Флаг | По умолчанию | Описание |
|------|--------------|----------|
| `--symbol` | `BTCUSDT` | Торговая пара (`ETH/USDT` → `ETHUSDT`) |
| `--interval` | `1m` | `1m`, `5m`, `1h`, `1d`, … |
| `--market` | `spot` | `spot` или `futures` (USD-M perpetual) |
| `--dir` | `data` | Каталог для CSV |
| `--since` | `2017-08-17` | Начало, UTC (`YYYY-MM-DD`) |
| `--until` | сейчас | Конец, UTC (`YYYY-MM-DD`) |
| `--no-progress` | `false` | Отключить progress bar (CI, логи) |

**Имена файлов:**

| Рынок | Файл |
|-------|------|
| spot | `data/BTCUSDT.csv` |
| futures | `data/BTCUSDT_futures.csv` |

**Колонки CSV:**

`open_time`, `open`, `high`, `low`, `close`, `volume`, `quote_volume`, `trades`, `taker_buy_base_volume`, `taker_buy_quote_volume`

**Примеры:**

```bash
# spot, минутки (долго на широком диапазоне — сужайте даты)
go run ./cmd/cli/... fetch-data --symbol BTCUSDT --interval 1m --since 2025-01-01 --until 2025-05-01

# futures, часовые свечи
go run ./cmd/cli/... fetch-data --symbol BTCUSDT --market futures --interval 1h --since 2023-01-01

# альткоин
go run ./cmd/cli/... fetch-data --symbol SOL/USDT --interval 1d
```

### `scan-markets`

Читает все `data/*.csv`. Для **каждой монеты** и **трёх периодов** перебирает цель продажи **1%…100%** (шаг 1%) и считает стратегию отдельно для каждого процента.

**Периоды:** весь файл / последние 2 года / **текущий год** (календарный год последней свечи в файле).

**Стратегии** (обе с перебором N=1–100%):

- **rise:** $100 → случайная минута → покупка → продажа при росте **+N%** → следующий день.
- **drop:** $100 → **шорт 1×** (случайная минута) → закрытие при падении **−N%** от цены входа → следующий день.
- **drop-margin:** шорт **2×**, маржа **$30** на сделку, перебор только **target 1–100%**; ликвидация при `close >= entry×(1+1/leverage)`.
- **trend:** **SMA(50)** на дневном close (вчера vs SMA); выше → **лонг** (+longTarget%), ниже → **шорт 1× $30** (−shortTarget%, ликвидация); перебор **long% 1–30 × short% 1–30**.
- **trend-long:** **SMA(50)** на дневном close; если тренд растущий (вчерашний close > SMA) — только **лонг** с выходом по **+N%**, перебор `N=1..100`.
- **trend-long-sma:** как trend-long, но перебор **SMA 10..100 шаг 10** × **long target 1..100** (цикл в цикле).

```bash
go run ./cmd/cli/... scan-markets
# только продажа на падении -N%
go run ./cmd/cli/... scan-markets --algo drop
# шорт + перебор плеча/маржи (долго: target×leverage×margin комбинаций)
go run ./cmd/cli/... scan-markets --algo drop-margin --target-max 20 --target-min 1
go run ./cmd/cli/... scan-markets --algo trend --target-max 30
go run ./cmd/cli/... scan-markets --algo trend-long --target-max 100
go run ./cmd/cli/... scan-markets --algo trend-long-sma
# одна монета + таблица best target по каждому SMA
go run ./cmd/cli/... scan-markets --algo trend-long-sma --symbol BTCUSDT --sma-report all
go run ./cmd/cli/... scan-markets --algo trend-long-sma-retest --symbol BTCUSDT --sma-report all
# CRT long (4H impulse + 15M entry; нужен volume в CSV)
go run ./cmd/cli/... scan-markets --algo crt-long --symbol BTCUSDT
# Breakout + Retest long (15M, volume не нужен)
go run ./cmd/cli/... scan-markets --algo breakout-retest-long --symbol BTCUSDT
go run ./cmd/cli/... scan-markets --algo breakout-retest-long-v2 --symbol BTCUSDT
# только продажа на росте +N%
go run ./cmd/cli/... scan-markets --algo rise
go run ./cmd/cli/... scan-markets --target-min 1 --target-max 100 --target-step 1
```

| Флаг | По умолчанию | Описание |
|------|--------------|----------|
| `--dir` | `data` | Каталог с CSV от `fetch-data` |
| `--seed` | `42` | База для RNG |
| `--last-years` | `2` | Длина периода «последние N лет» |
| `--target-min` | `1` | Мин. % для продажи |
| `--target-max` | `100` | Макс. % для продажи |
| `--target-step` | `1` | Шаг перебора % |
| `--symbol` | — | Только одна монета (например `BTCUSDT`) |
| `--sma-report` | `best` | Для SMA-алго: `best` или `all` (таблица по каждому SMA) |
| `--algo` | `all` | `rise`, `drop`, `drop-margin`, `trend`, `trend-long` или `all` (= rise+drop) |
| drop-margin | 2×, $30 | Плечо и маржа фиксированы в коде |
| trend | SMA(50) 1d | Long/short target sweep; шорт $30 1× + liq |
| trend-long | SMA(50) 1d | Long-only target sweep в bullish regime |
| trend-long-sma | SMA 10–100 step 10 | Long-only; sweep SMA × target |

После каждого периода: две таблицы (rise и drop), строка `>> best target`; в конце — сводка по монетам и алгоритмам.

> **Rate limit:** между запросами пауза ~200 ms, по 1000 свечей за запрос; при таймаутах — до 5 повторов. Данные пишутся в `data/<pair>.csv.tmp` и переименовываются в `.csv` только после успешной загрузки. Полные минутки за годы — часы; для длинной истории используйте `--interval 1h`/`1d` или готовые архивы (Kaggle, CryptoDataDownload).

### `fear-research`

Читает CSV, сортирует по дате, перебирает пороги Fear & Greed и варианты выхода по % цены. Ищет комбинации с максимальным условным результатом от $100. Содержит отладочный вывод — логика экспериментальная.

По умолчанию в коде открывается `btc_fear_greed_hour_eth.csv` в **корне** репозитория (другие пути закомментированы в `fear_research.go`).

### `hello-world`

Проверка запуска CLI.

## Переменные окружения

| Переменная | Описание |
|------------|----------|
| `APP_NAME` | Имя CLI (по умолчанию `fead-and-greed`) |
| `TG_BOT_TOKEN` | Токен Telegram-бота |
| `TG_CHAT_ID` | ID чата для уведомлений |

Файл `.env` опционален; при отсутствии в лог пишется предупреждение.

## Данные

### Каталог `data/` (Go `fetch-data`)

Актуальный формат для загрузки с Binance. Не коммитится в git.

### Legacy CSV в корне (PHP / ручные выгрузки)

Используются командой `fear-research` и скриптами `fear8*.php`.

Дневной формат (`btc_fear_greed.csv`):

```csv
date,fear_greed_index,btc_price_usd
2025-07-03,73,109299.59
```

Прочие файлы в корне:

- `btc_fear_greed_hour*.csv` — почасовые / минутные срезы
- `btc_fear_greed_hour_eth.csv` — ETH + F&G (часто с `fear=0` в hour-скрипте)
- `*_minutes*`, `*_bck`, `*_part*` — бэкапы и части дампов

**Fear & Greed** доступен только с [alternative.me](https://api.alternative.me/fng/) (дневной индекс на весь рынок, не per-coin). В `fetch-data` индекс не подмешивается — только биржевые свечи.

## Сбор данных (PHP, legacy)

| Скрипт | Назначение |
|--------|------------|
| `fear8.php` | F&G + дневные цены BTC (Binance) → `btc_fear_greed.csv` |
| `fear8_hour.php` | Минутные klines (Binance), опционально ETH → `btc_fear_greed_hour_*.csv` |

```bash
php fear8.php
php fear8_hour.php
```

Для новых загрузок предпочтительнее **`fetch-data`** на Go: пагинация, spot/futures, единый формат OHLCV.

## Структура проекта

```
cmd/cli/                         # точка входа
internal/
  presentation/command/          # CLI-команды
  app/cli/
    usecase/                     # HelloWorld, FearResearch, FetchData
    config/                      # конфиг из env
    registry/                    # DI-контейнер
    cli.go
  infrastructure/binance/        # клиент klines (spot / futures)
  domain/model/                  # DayInfo
pkg/
  env/
  logger/
  wrap/
  telegram/
data/                            # CSV от fetch-data (.gitignore)
fear8.php, fear8_hour.php        # legacy-сбор данных
```

## Makefile

| Цель | Действие |
|------|----------|
| `make tidy` | `go mod tidy` |
| `make unit-test` | тесты `./internal/...`, `./pkg/...` |
| `make lint` | golangci-lint |
| `make build` | сборка в Docker (`golang:1.22`) |

## CI

GitHub Actions (`.github/workflows/go.yml`): `go build` и `go test` на push/PR в `main`.

## Зависимости

- Go 1.22+
- [urfave/cli v2](https://github.com/urfave/cli) — CLI
- [godotenv](https://github.com/joho/godotenv) — `.env`
- [go-resty](https://github.com/go-resty/resty) — HTTP для Telegram
- [tracerr](https://github.com/ztrue/tracerr) — ошибки со стеком

Загрузка Binance klines — стандартная библиотека `net/http`, без ccxt.

## Состояние разработки

- [x] `fetch-data`: spot и USD-M futures
- [ ] Связать `fear-research` с CSV из `data/` и расширенными колонками
- [ ] Подмешивание Fear & Greed в пайплайн загрузки
- [ ] Telegram-уведомления из usecase
- [ ] Убрать отладочный вывод в `fear_research`
