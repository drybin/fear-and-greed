# Алгоритмы backtest (`scan-markets`)

Документ описывает, как в проекте `fear-and-greed` устроены входные данные, симуляция сделок и перебор параметров. Цель — воспроизвести логику в другом проекте без чтения Go-кода.

---

## 1. Общая схема

```
CSV (минутки) → фильтр периода → симулятор стратегии → отчёт / sweep best
```

Команда: `scan-markets`

- Читает все `*.csv` из каталога `data/` (или одну монету через `--symbol`).
- Для каждой монеты прогоняет **3 периода** (если достаточно свечей):
  1. **full** — весь ряд;
  2. **last_2_years** — последние N лет (флаг `--last-years`, по умолчанию 2);
  3. **current_year** — календарный год последней свечи.
- Для выбранного `--algo` перебирает параметры (target %, SMA, …) и выбирает **best по `ProfitPct`**.
- В конце печатает summary по монетам/периодам/алгоритмам.

---

## 2. Входные данные

### 2.1. Формат CSV

Файл создаётся командой `fetch-data`. Минимум 5 колонок:

| Колонка | Поле      | Формат |
|---------|-----------|--------|
| 0       | `open_time` | `2006-01-02 15:04:05`, **UTC** |
| 1       | `open`    | float |
| 2       | `high`    | float |
| 3       | `low`     | float |
| 4       | `close`   | float |

Первая строка — заголовок, дальше свечи **отсортированы по времени**.

Имя файла: `BTCUSDT.csv` → символ `BTCUSDT`.

### 2.2. Структура свечи в памяти

```text
Candle {
  OpenTime time.Time  // UTC
  Open, High, Low, Close float64
}
```

### 2.3. Требования к ряду

- Интервал в данных — обычно **1 минута** (алгоритм не проверяет шаг; работает с любым, но логика «случайная минута дня» предполагает минутки).
- Свечи идут подряд по времени внутри дня.
- Для SMA(N) нужно минимум N календарных дней с дневным close до первого сигнала.

---

## 3. Общие правила симуляции

### 3.1. Капитал

- Стартовый депозит: **`StartCash = 100` USD**.
- Long: на вход **весь текущий cash** покупает монету: `coins = cash / entryPrice`.
- Одновременно **не больше одной позиции** (long или short).

### 3.2. Long — вход и выход

**Вход:** по `close` свечи входа.

**Выход (take-profit):** на каждой следующей свече проверяется:

```text
targetPrice = entryPrice * (1 + targetPct / 100)
if candle.Close >= targetPrice:
    exit at candle.Close
    cash = coins * exitPrice
```

Stop-loss **нет**. Если target не достигнут до конца данных — позиция остаётся **открытой**.

### 3.3. Переход к следующей сделке

После закрытия long (или short cover / liquidation в других алго):

```text
nextDay = первый календарный день ПОСЛЕ дня выхода, для которого есть свечи
i = индекс первой свечи nextDay
```

Нельзя открыть новую сделку в тот же календарный день, что и выход.

### 3.4. Случайный вход внутри дня

На «бычьем» / eligible дне:

```text
eligible = все индексы свечей текущего календарного дня, начиная с текущего i
entryIdx = eligible[ randomInt(0, len(eligible)-1) ]   // rng от seed
entryPrice = candles[entryIdx].Close
```

RNG: `rand.New(rand.NewSource(seed))`, один seed на комбинацию `(symbol, period, algo, params…)`.

### 3.5. Отчёт (`SimulationReport`)

| Поле | Смысл |
|------|--------|
| `RealizedCash` | cash после последней **закрытой** сделки |
| `ProfitUSD` | `RealizedCash - StartCash` |
| `ProfitPct` | `(RealizedCash / StartCash - 1) * 100` |
| `CompletedCount` | число закрытых round-trip |
| `OpenPosition` | в конце ряда позиция ещё открыта |
| `FinalCash` | MTM в конце (для open long: `coins * lastClose`) |
| `OpenLegUSD` | `FinalCash - RealizedCash` при открытой позиции |

**Важно:** в sweep и summary «profit» = только **реализованный** P/L. Нереализованный хвост — отдельно `[open leg $… MTM]`.

### 3.6. Дневной close и SMA

**Календарный день:** `truncateDay(t) = Date(Y, M, D, 0, 0, 0, UTC)`.

**Дневной close:** для каждого дня D — `close` **последней** минутной свечи этого дня (перезаписывается при обходе).

**SMA(period)** на день D:

```text
SMA(D) = mean( dailyClose[D-period+1], …, dailyClose[D] )
```

Дни сортируются по календарю. SMA есть только начиная с day index `period-1`.

### 3.7. Trend-фильтр (bull / bear / skip)

Для **календарного дня входа** `entryDay`:

```text
prev = entryDay - 1 calendar day
closePrev = dailyClose[prev]
smaPrev   = dailySMA[prev]

if closePrev > smaPrev  → bull = true
if closePrev < smaPrev  → bull = false
if closePrev == smaPrev → skip (нет входа в этот день)
if нет данных           → skip
```

Это **не** crossover на минутках: режим определяется **вчерашним дневным close vs вчерашним SMA**.

---

## 4. Seed (детерминизм)

Базовый seed: `--seed` (default `42`).

Для каждого прогона:

```text
h = FNV-1a 64-bit
h.write(symbol)
h.write(0)
h.write(period)          // "full" | "last_2_years" | "current_year"
h.write(0)
h.write(algoName)      // см. ниже
h.write(0)
h.write(paramsString)  // например "40:12" для sma:target
seed = int64(h.sum64()) XOR baseSeed
```

Примеры `algoName`:

| Алгоритм | строка в hash |
|----------|----------------|
| rise / drop | `"rise"` / `"drop"` |
| trend-long-sma | `"trend-long-sma"` + `"%d:%d"` (sma, target) |
| trend-long-sma-retest | `"trend-long-sma-retest"` + `"%d:%d"` |

---

## 5. Алгоритмы

### 5.1. `rise` — random long + take-profit

- Каждый день: случайная минута → long на весь cash.
- Выход: `close >= entry * (1 + N%)`.
- Sweep: **N = 1..100** (шаг `--target-step`).

Псевдокод совпадает с §3.2–3.4 без trend-фильтра.

---

### 5.1b. `rise-2d-profit` — random long + 2D / target

**Файл:** `internal/strategy/random_rise_2d.go`

Как `rise`: каждый день случайная минута → long на весь cash. **Стопа нет.**

| Фаза | Выход |
|------|--------|
| **До 48h** | `close >= entry × (1 + target%)` → `exit_reason: target` |
| **После 48h** | `close >= entry + 2%` → `profit_2d` или `profit_wait` (если цена уже опускалась ниже +2% после 48h) |
| После 48h, пока нет +2% | позиция **держится** (без стопа) |

Sweep **target 1..100%** (как `rise`). Seed — FNV от symbol/period/algo/target.

#### CLI

```bash
go run ./cmd/cli/... scan-markets --algo rise-2d-profit --symbol BTCUSDT
```

Алиасы: `rise-2d`, `rise2d`.

---

### 5.2. `drop` — random short 1× + cover

- Каждый день: случайная минута → short **1×**, notional = весь cash.
- Cover: `close <= entry * (1 - N%)`.
- P/L short 1×: `cash += notional * (entry - mark) / entry`.

---

### 5.3. `drop-margin` — short с плечом

- Short **2×**, маржа **$30** на сделку (фиксировано в коде).
- Ликвидация: `close >= entry * (1 + 1/leverage)`.
- Cover: `close <= entry * (1 - N%)`.
- Sweep target 1..100%.

Формулы:

```text
liquidationPrice = entry * (1 + 1/leverage)
shortPnLUSD(margin, leverage, entry, mark) = margin * leverage * (entry - mark) / entry
```

---

### 5.4. `trend` — SMA(50) long + short

- SMA period = **50** (фиксировано).
- Bull day → long (весь cash), exit `+longTarget%`.
- Bear day → short $30, leverage 1×, exit `-shortTarget%` или liquidation.
- Sweep: long 1..30 × short 1..30 (каждый capped at 30 даже если `--target-max` больше).

---

### 5.5. `trend-long` — SMA(50) long only

- SMA(50), только **bull** дни.
- Вход: случайная минута bull-дня.
- Выход: `+target%`.
- Sweep target 1..100.

Логика = §3.7 + §3.4 + §3.2.

---

### 5.6. `trend-long-sma` — sweep SMA × target

**Параметры sweep (фиксированы в коде):**

| Параметр | Значение |
|----------|----------|
| SMA | 10, 20, 30, …, 100 (шаг 10) |
| long target | `--target-min` .. `--target-max` (default 1..100) |

**На каждую пару (SMA, target):**

1. Построить `dailyClose`, `dailySMA(period)`.
2. Пройти минутки с `i = 0`:

```text
if not inPosition:
    if not bull(entryDay): skip to next day; continue
    pick random minute on entryDay
    enter long at close
    i = entryIdx + 1
else:
    if close >= entry * (1 + target/100):
        close position at close
        jump to next calendar day after exit
    else:
        i++
```

3. Best выбирается по **максимальному `ProfitPct`** среди всех (SMA × target).

**Оптимизация в коде (не меняет результат):** для фиксированного SMA один проход по свечам считает все target сразу (`SimulateTrendLongOnlySMASweepWithCache`).

**CLI:**

```bash
scan-markets --algo trend-long-sma
scan-markets --algo trend-long-sma --symbol BTCUSDT --sma-report all
```

`--sma-report all` — таблица **best target на каждый SMA** + общий best.

---

### 5.7. `trend-long-sma-retest` — breakout + retest + long

**Доп. параметры:**

| Параметр | CLI | Default |
|----------|-----|---------|
| `epsilonPct` | `--retest-epsilon` | `0.1` (% вокруг SMA) |
| `lookahead` | `--retest-lookahead` | `60` (свечей после breakout) |

**Sweep:** те же SMA 10..100 step 10 × target 1..100.

#### Предусловие (как у trend-long)

День `D` eligible только если **bull** по §3.7 (вчера close > SMA).

#### SMA для минутного сигнала

На свече с индексом `i`, день `D = truncateDay(candles[i].OpenTime)`:

```text
sma = dailySMA[ D - 1 calendar day ]   // SMA «вчера», та же база что trend-фильтр
```

#### Состояния

```text
waitRetest = false
retestUntil = -1
inPosition = false
```

Цикл с **`i = 1`** (нужен `candles[i-1]` для breakout).

#### Шаг A — не в позиции, не ждём retest

```text
if not bull(D): reset waitRetest; i++; continue
if no sma: reset; i++; continue

prevClose = candles[i-1].Close
close     = candles[i].Close

if prevClose <= sma AND close > sma:
    waitRetest = true
    retestUntil = i + lookahead    // inclusive window: i+1 .. retestUntil
i++
```

Breakout строго по **close** минуток относительно **одного и того же** `sma`.

#### Шаг B — ждём retest

```text
if i > retestUntil:
    reset waitRetest; continue   // без i++ в некоторых ветках — в коде continue после reset

touchMin = sma * (1 - epsilonPct/100)
touchMax = sma * (1 + epsilonPct/100)
touched = (low <= touchMax) AND (high >= touchMin)

if touched:
    entryPrice = candles[i].Close
    enter long (coins = cash / entryPrice)
    reset waitRetest
    i++
else:
    i++
```

Касание — по **high/low** свечи в зоне `[touchMin, touchMax]`.

#### Шаг C — в позиции

Как обычный long TP:

```text
if close >= entry * (1 + target/100):
    exit at close
    advance to next calendar day (i >= 1)
```

#### Особенности

- **Short нет.**
- RNG seed передаётся, но **вход детерминирован** правилами (seed сейчас не влияет на retest-вход).
- Закрытые сделки при `target > 0` всегда с profit ≥ target% от entry.
- Убыточным может быть только **open leg** в конце периода.

---

## 6. Фильтры периода

```text
FilterLastYears(candles, years):
    cutoff = lastOpenTime - years * 365.25 days
    keep candles where OpenTime >= cutoff

FilterCurrentYear(candles):
    y = year of last candle
    keep candles where OpenTime >= Jan 1 of year y
```

---

## 7. Sweep и выбор best

### 7.1. Общий принцип

Для каждой комбинации параметров — полная симуляция → `ProfitPct`.

Best = комбинация с **максимальным `ProfitPct`**.

Флаг `--min-trades` (default 1): в summary best учитывается только если `CompletedCount >= minTrades` (в таблице sweep warning, если best < minTrades).

### 7.2. `trend-long-sma` / retest

```text
for sma in 10,20,...,100:
    for target in targetMin..targetMax step targetStep:
        run simulation
        track best per (sma, target)
    track best target for this sma   // для --sma-report all
track global best across all sma×target
```

---

### 5.8. `crt-long` — Candle Range Theory (4H + 15M)

**Файлы:** `internal/strategy/crt.go`, `aggregate.go`, `indicators.go`

**Вход:** минутные свечи с **volume** в CSV (колонка 6). Без volume импульсы не детектятся.

#### Агрегация (UTC)

| TF | Минут | Bucket |
|----|-------|--------|
| 4H | 240 | час кратен 4 (0,4,8,12,16,20) |
| 15M | 15 | минуты 0,15,30,45 |

`volume` = сумма минутных объёмов в bucket.

#### Импульс (на закрытии 4H-бара `k`)

```text
bullish      = close > open
range        = high - low
range > ATR14(k) * 1.5
volume > SMA(volume, 20) * 1.5
high > max(high[k-20 .. k-1])
```

→ `rangeHigh`, `rangeLow`, `eq = (high+low)/2`, состояние `WAIT_RETEST`.

Новый 4H-импульс **игнорируется**, пока активен сетап (`WAIT_RETEST` / `WAIT_CONFIRM` / `IN_POSITION`).

#### FSM

```text
IDLE → WAIT_RETEST → WAIT_CONFIRM → IN_POSITION → IDLE

Терминалы без сделки: CANCEL (close15 < rangeLow до входа), EXPIRE
Терминалы со сделкой: STOP, BE_STOP, TP2_DONE, open leg в конце данных
```

**Expire:** на закрытии 4H-бара `h`, если `h >= impulseIdx + 12` и за всё время не было 15M в discount → сброс.

**Discount (15M):** `low <= eq AND high >= rangeLow`.

**Вход:** в `WAIT_CONFIRM`, в discount, бычья реакция:

```text
close > open AND close > prevClose
→ entry = close, coins = cash / entry
```

#### В позиции (15M)

**До TP1:**

- `close < rangeLow` → **stop**, 100% exit.
- `high >= rangeHigh` → продать **50%** по `rangeHigh`, SL оставшейся части → **breakeven** (`entry`).

**После TP1:**

- `close < entry` → выход оставших 50% (BE stop).
- **TP2** = ближайший swing high выше `rangeHigh` (pivot `P=2` на 15M, минимальный `high` среди pivot > rangeHigh).
- Если swing не найден: `tpFallback = entry + 2.0 * (entry - rangeLow)`.
- `high >= target2` → закрыть остаток.

#### P/L

- Старт $100, реинвест полного cash в каждый вход.
- `ProfitPct` от `RealizedCash` (после всех частичных фиксаций).
- `CompletedCount` = число полностью закрытых CRT-сетапов (1 Trade на полное закрытие).

#### CLI

```bash
go run ./cmd/cli/... scan-markets --algo crt-long --symbol BTCUSDT
```

Параметры **фиксированы** в коде (`CRTATRMult`, `CRTVolMult`, …); sweep не используется.

---

### 5.9. `breakout-retest-long` — Swing breakout + retest (15M)

**Файлы:** `internal/strategy/breakout_retest.go`

**Volume не нужен.** Минутки → 15M.

#### Swing (N=3)

Pivot high/low: строго выше/ниже `N` соседей с каждой стороны.  
**Breakout level** = `high` последнего подтверждённого swing high **до** бара пробоя.

**Breakout:** `close > breakoutLevel`.

#### Impulse

```text
impulseLow  = последний swingLow перед пробоем (fallback: low breakout-свечи)
impulseHigh = high breakout-свечи
```

#### Retest zone (на breakout-баре)

```text
buffer = ATR14(15M) * 0.15
zoneTop / zoneBottom = breakoutLevel ± buffer
```

Touch: `low <= zoneTop AND high >= zoneBottom AND close >= zoneBottom`.

#### FSM

```text
IDLE → WAIT_RETEST → WAIT_CONFIRM → IN_POSITION → IDLE
```

- Retest/вход только на барах **после** breakout-бара.
- **Expire:** 12×15M без touch → cancel.
- **Cancel:** `close < zoneBottom` до входа.
- **Entry:** после touch, `close >= breakoutLevel`, bullish (`close > open`).
- **SL:** `breakoutLevel - ATR×0.5`; выход при `close < SL` (100% до TP1).
- **TP1:** 50% @ `impulseHigh`, затем **breakeven**.
- **TP2:** swing high > impulseHigh или `entry + 2R`.
- Один активный сетап; новый breakout игнорируется.

#### CLI

```bash
go run ./cmd/cli/... scan-markets --algo breakout-retest-long --symbol BTCUSDT
```

Алиасы: `breakout-retest`, `br-retest`.

---

### 5.10. `breakout-retest-long-v2`

**Файл:** `internal/strategy/breakout_retest_v2.go`

**Нужен volume в CSV.**

| Блок | Правило |
|------|---------|
| Trend | 15M `close > EMA200` и **закрытый** 1H `close > EMA200` |
| Breakout | swing high N=**5**, `close > swingHigh + ATR×0.2`, `volume > SMA(20)×1.3` |
| Retest | бары **3–12** после breakout; touch zone ±ATR×0.15; `close > swingHigh` |
| Entry | **open** следующей свечи после confirm |
| Cancel | `close < swingHigh - ATR×0.2` до входа |
| SL | `retestLow - ATR×0.1` |
| TP | **1R** (50%), BE, **2R** (50%) |

```bash
go run ./cmd/cli/... scan-markets --algo breakout-retest-long-v2 --symbol BTCUSDT
```

Алиасы: `breakout-retest-v2`, `br-retest-v2`.

---

### 5.11. `fib-pullback-long` (v1)

**Файл:** `internal/strategy/fib_pullback.go`

**Volume не обязателен** (volumeRatio логируется при входе).

| Блок | Правило |
|------|---------|
| Leg TF | **1H**, swing N=**5** |
| Entry TF | **15M** |
| Trend | 1H `close > EMA200`, 15M `close > EMA200`, 1H `EMA200[i] > EMA200[i-20]` |
| Leg | последний swingLow → swingHigh; BOS = 1H close **впервые** пробивает **предыдущий** swingHigh |
| Impulse | `(high-low)/low >= 6%` |
| Fib zone | **0.5–0.618** (от impulseHigh вниз) |
| Entry | touch zone → `close > EMA20(15M)` и `close > high[i-1]`; вход по **close** |
| Cancel | `close < fib 0.786` или **>48×15M** после BOS |
| SL | ниже fib **0.786** или swingLow (tighter = `max`, если risk ≥ 0.4% entry) |
| TP | **1R** (50%), BE, **2R** (50%) |

#### FSM

```text
IDLE → WAIT_TOUCH → WAIT_CONFIRM → IN_POSITION → IDLE
```

- Структура и BOS на **закрытии 1H**; вход/выход на **15M**.
- Один активный сетап; новый BOS игнорируется.

#### CLI

```bash
go run ./cmd/cli/... scan-markets --algo fib-pullback-long --symbol BTCUSDT
```

Алиасы: `fib-pullback`, `fib-long`.

---

### 5.12. `fib-pullback-long-v2`

**Файл:** `internal/strategy/fib_pullback_v2.go`

**Нужен volume в CSV** (фильтр BOS на 1H).

| Блок | Правило |
|------|---------|
| База | как v1: 1H leg + 15M entry, fib **0.5–0.618** |
| BOS | `close > prevSwingHigh + ATR×0.2` (15M ATR на close 1H) |
| Volume | 1H `volume > SMA(20)×1.2` на BOS (пропуск если volume=0 в CSV) |
| Impulse | default **8%**, sweep **6 / 8 / 10%** |
| Cancel | `close < fib 0.786` (как v1) |
| Entry | touch bar **≠** confirm bar; `close > EMA20`; `close > max(high[i-3..i-1])`; bullish |
| Confirm window | ≤ **24×15M** после touch |
| Risk | skip bar если `(entry-SL)/entry > 4%` (сетап живёт) |
| Cooldown | **24×15M** после закрытия сделки |
| TP1 | **50% @ impulseHigh** |
| После TP1 | trail: exit если `close < EMA20(15M)` |
| TP2 | **2R** на остаток |

#### CLI

```bash
go run ./cmd/cli/... scan-markets --algo fib-pullback-long-v2 --symbol BTCUSDT
```

Алиасы: `fib-pullback-v2`, `fib-v2`.

---

### 5.13. `fib-pullback-trend-v1` — spec BOS + fib retest

**Файл:** `internal/strategy/fib_pullback_trend_v1.go`

Отдельная реализация **по спецификации** (не путать с `fib-pullback-long` v1 — там другие BOS/SL/тренд).

| Блок | Правило |
|------|---------|
| Leg TF | **1H**, pivot N (default **5**) |
| Entry TF | **15M** |
| Trend | **только 1H**: `close > EMA200`, `EMA200[i] > EMA200[i-20]` (+ sweep EMA50-фильтры) |
| BOS | `1H close > last swing high`; `legHigh` = **high BOS-бара** |
| Impulse | `(legHigh-legLow)/legLow >= min%` (default **8%**) |
| Fib zone | touch: `low <= zoneTop` и `high >= zoneBottom` (default **0.5–0.618**) |
| Cancel | `15M close < fib786` или timeout (**48×15M** default) |
| Confirm | `close > EMA20(15M)` и `close > prevHigh` |
| SL | строго **fib786** |
| TP | **1R** (50%), BE по **low**, **2R** (50%) |

#### Sweep (108 combo)

`minImpulse` 5/8/10/15% × `pivot` 3/5/7 × zone 0.382–0.618 / 0.5–0.618 / 0.5–0.786 × `maxWait` 24/48/96×15M.

#### FSM

```text
IDLE → WAIT_PULLBACK → WAIT_CONFIRM → IN_POSITION → IDLE
```

#### CLI

```bash
go run ./cmd/cli/... scan-markets --algo fib-pullback-trend-v1 --symbol BTCUSDT
```

Алиасы: `fib-trend-v1`, `fpt-v1`.

---

### 5.14. `nr7-trend-breakout-v1` — NR compression breakout

**Файл:** `internal/strategy/nr7_trend_breakout_v1.go`

| Блок | Правило |
|------|---------|
| TF | **1H** |
| Trend | rising EMA200 + sweep: `close>EMA200` / `EMA50>EMA200` / оба |
| Compression | range свечи = **min** за N баров (default **7**) и `range < ATR14 × mult` (default **0.8**) |
| Setup | сохранить `nrHigh`, `nrLow`; ждать пробой ≤ **12** баров (sweep 6/12/24) |
| Entry | `close > nrHigh` и `range > ATR14` |
| SL | `nrLow` |
| TP | **1R** (50%), BE по **low**, **2R** |

#### Sweep (81 combo)

NR length 5/7/10 × ATR mult 0.6/0.8/1.0 × lifetime 6/12/24 × trend filter ×3.

#### FSM

```text
IDLE → WAIT_BREAKOUT → IN_POSITION → IDLE
```

#### CLI

```bash
go run ./cmd/cli/... scan-markets --algo nr7-trend-breakout-v1 --symbol BTCUSDT
```

Алиасы: `nr7`, `nr7-breakout`, `nr-breakout`.

---

### 5.15. `volatility-compression-breakout-v1` — ATR-min compression breakout

**Файл:** `internal/strategy/volatility_compression_breakout_v1.go`

| Блок | Правило |
|------|---------|
| TF | **1H** |
| Trend | rising EMA200 + sweep EMA50-фильтры (как NR7) |
| Compression | `ATR14` = **min** за окно (default **100**) и `atr < avg(window) × factor` (default **0.6**) |
| Range | `compressionHigh/Low` = high/low за **10** баров (sweep 5/10/20) |
| Lifetime | **24×1H** |
| Entry | `close > compressionHigh` и `range > ATR14 × expansion` (default **1.5**) |
| SL | `compressionLow` (опц. режим `entry - ATR×2` — не в sweep) |
| TP | **1R** (50%), BE, **2R** |

#### Sweep (324 combo)

comp window 50/100/200 × range 5/10/20 × ATR factor 0.5–0.8 × expansion 1.2/1.5/2.0 × trend ×3.

#### CLI

```bash
go run ./cmd/cli/... scan-markets --algo volatility-compression-breakout-v1 --symbol BTCUSDT
```

Алиасы: `vcb-v1`, `vcb`, `vol-compression-breakout`.

---

### 5.16. `liquidity-sweep-long` (v1)

**Файл:** `internal/strategy/liquidity_sweep_long.go`

| Блок | Правило |
|------|---------|
| TF | **1H** |
| Trend | `close > EMA200` |
| Level | `priorLow = min(low, 20)` |
| Sweep | `low < priorLow` и `close > priorLow` |
| Confirm | **следующий** бар: `close > sweepHigh` |
| SL | `sweepLow` |
| TP | **2R** (полная позиция, без partial TP1) |
| Cooldown | **12×1H** после сделки |

Алиасы: `liquidity-sweep`, `sweep-long`, `lsl`.

---

### 5.17. `liquidity-sweep-long-v2` — swing pivot sweep

**Файл:** `internal/strategy/liquidity_sweep_v2_long.go`

| Блок | Правило |
|------|---------|
| Level | swing pivot low **N=2** (живёт до **48** баров) |
| Sweep | прокол pivot + `close` выше уровня |
| Confirm | 1 бар: `close > sweepHigh` |
| SL / TP | sweep low / **2R** |
| Выход stop | по **close** (не low) |

---

### 5.18. `liquidity-sweep-long-v3` — equal lows pool

**Файл:** `internal/strategy/liquidity_sweep_v3_long.go`

| Блок | Правило |
|------|---------|
| Pool | два pivot low в **±0.2%**, разделение ≤ **24** бара |
| Sweep + confirm | как v2 |
| SL / TP | sweep low / **2R** |

Алиасы: `equal-lows`, `lsl-v3`.

---

### 5.19. `liquidity-sweep-long-v4` — sweep + displacement

**Файл:** `internal/strategy/liquidity_sweep_v4_long.go`

| Блок | Правило |
|------|---------|
| Sweep | как v2 (pivot N=2) |
| Entry | **displacement**: бычья свеча, `range > ATR14×1.5` (на баре sweep или ≤ **3** бара) |
| SL / TP | sweep low / **2R** |

Алиасы: `displacement`, `lsl-v4`.

---

### 5.20. `liquidity-sweep-long-v5` — sweep + FVG retest

**Файл:** `internal/strategy/liquidity_sweep_v5_long.go`

| Блок | Правило |
|------|---------|
| Sweep | как v2 |
| FVG | bullish FVG после displacement (≤ **5** баров): `low[i] > high[i-2]` |
| Entry | ретест FVG (≤ **12** баров): касание зоны + бычье закрытие |
| SL / TP | sweep low / **2R** |

Алиасы: `fvg`, `lsl-v5`.

#### CLI (все liquidity sweep)

```bash
go run ./cmd/cli/... scan-markets --algo liquidity-sweep-long-v4 --symbol BTCUSDT
```

---

## 8. Минимальный план портирования

1. Загрузчик CSV → массив `Candle`.
2. `buildDailyCloses` + `buildDailySMA(period)`.
3. `trendRegime(entryDay)` → bull/bear/skip.
4. Симулятор long с одной позицией + `advanceAfterExit`.
5. Обёртка sweep + FNV seed как в §4.
6. Для retest — конечный автомат из §5.7.

### Проверочные инварианты

- Long TP: каждая закрытая сделка `(exit/entry - 1) * 100 >= target`.
- После exit новый вход не раньше **следующего календарного дня**.
- `ProfitPct` считается от `RealizedCash`, не от `FinalCash`.
- SMA на день D использует только closes дней `<= D`.

---

## 9. Ссылки на код

| Что | Файл |
|-----|------|
| CSV | `internal/infrastructure/csvdata/klines.go` |
| Long / rise / drop | `internal/strategy/random_five_percent.go` |
| Rise 2d profit | `internal/strategy/random_rise_2d.go` |
| Trend, SMA, retest | `internal/strategy/trend.go` |
| Short margin | `internal/strategy/short_leveraged.go` |
| Sweep orchestration | `internal/app/cli/usecase/scan_markets.go` |
| SMA sweep | `internal/app/cli/usecase/scan_trend_long_sma_sweep.go` |
| Retest sweep | `internal/app/cli/usecase/scan_trend_long_sma_retest_sweep.go` |
| CRT long | `internal/strategy/crt.go` |
| CRT scan | `internal/app/cli/usecase/scan_crt_long.go` |
| Breakout retest | `internal/strategy/breakout_retest.go` |
| Breakout retest scan | `internal/app/cli/usecase/scan_breakout_retest_long.go` |
| Breakout retest v2 | `internal/strategy/breakout_retest_v2.go` |
| Breakout retest v2 scan | `internal/app/cli/usecase/scan_breakout_retest_v2_long.go` |
| Fib pullback long | `internal/strategy/fib_pullback.go` |
| Fib pullback scan | `internal/app/cli/usecase/scan_fib_pullback_long.go` |
| Fib pullback v2 | `internal/strategy/fib_pullback_v2.go` |
| Fib pullback v2 scan | `internal/app/cli/usecase/scan_fib_pullback_v2_long.go` |
| Fib pullback trend v1 | `internal/strategy/fib_pullback_trend_v1.go` |
| Fib pullback trend scan | `internal/app/cli/usecase/scan_fib_pullback_trend_v1.go` |
| NR7 trend breakout v1 | `internal/strategy/nr7_trend_breakout_v1.go` |
| NR7 scan | `internal/app/cli/usecase/scan_nr7_trend_breakout_v1.go` |
| Vol compression breakout v1 | `internal/strategy/volatility_compression_breakout_v1.go` |
| VCB scan | `internal/app/cli/usecase/scan_volatility_compression_breakout_v1.go` |
| Liquidity sweep v1 | `internal/strategy/liquidity_sweep_long.go` |
| Liquidity sweep v2–v5 | `internal/strategy/liquidity_sweep_v2_long.go` … `v5_long.go` |
| Liquidity sweep scan | `internal/app/cli/usecase/scan_liquidity_sweep_*.go` |
| Scan reports | `internal/infrastructure/scanreport/` |

---

## 11. HTML-отчёты (`--report-dir`, `--html`)

После прогона `scan-markets` можно сохранить JSON и сгенерировать HTML для сравнения алгоритмов.

### Структура

```text
reports/
  algorithms.json       # описания алгоритмов (карточки в HTML)
  manifest.json         # метаданные последнего прогона
  report.html           # сравнение (если --html)
  data/
    <algo>/
      BTCUSDT__full.json
      BTCUSDT__last_2_years.json
      ...
```

При повторном запуске `--algo X` каталог `data/X/` **очищается** и перезаписывается.

### CLI

```bash
# JSON + HTML
go run ./cmd/cli/... scan-markets \
  --algo fib-pullback-long-v2 \
  --report-dir reports \
  --html true

# только JSON
go run ./cmd/cli/... scan-markets --algo crt-long --report-dir reports
```

HTML: фильтры **период / символ**, режимы сравнения, сортировка по profit / сделкам. Клик по строке → карточка алгоритма и **таблица сделок** (`trades[]`: время, цены, `exit_reason`, `entry_context`, `exit_context`, `events`).

### Поле `trades` в JSON

```json
"trades": [
  {
    "entry_time": "2024-03-15T10:30:00Z",
    "exit_time": "2024-03-15T18:45:00Z",
    "entry_price": 67234.5,
    "exit_price": 68100,
    "pnl_pct": 1.29,
    "exit_reason": "tp2",
    "entry_context": { "fib50": 67000, "sl": 66200, "ema20_15m": 67100 },
    "exit_context": { "close": 68100, "tp2": 68000, "tp1_done": 1 },
    "events": [
      { "kind": "tp1_partial", "time": "2024-03-15T14:00:00Z", "price": 67500, "fraction": 0.5 }
    ]
  }
]
```

`exit_reason`: `stop` | `breakeven` | `tp2` | `trail_ema20` | `target` | `cover`.  
`pnl_pct` — **blended** PnL при наличии `events` (50% на TP1 + остаток на финальном выходе).  
`entry_context` — fib / breakout-retest / CRT; `exit_context` — уровни на финальном выходе; `events` — частичные фиксации (`tp1_partial`).

---

## 10. Пример CLI

```bash
# SMA sweep + таблица по каждому SMA, одна монета
go run ./cmd/cli/... scan-markets \
  --algo trend-long-sma \
  --symbol BTCUSDT \
  --sma-report all \
  --target-min 1 --target-max 100 --target-step 1 \
  --seed 42

# Breakout + retest
go run ./cmd/cli/... scan-markets \
  --algo trend-long-sma-retest \
  --symbol BTCUSDT \
  --sma-report all \
  --retest-epsilon 0.1 \
  --retest-lookahead 60
```
