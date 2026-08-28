# Сводка batch-прогонов: top-50 и CMC #50–200

Документ фиксирует выводы двух полных прогонов `scan-markets` (все алгоритмы, best-of-sweep по `profit_pct`) и их сравнение.

| | Batch 1 | Batch 2 |
|--|---------|---------|
| Источник результатов | `/Users/d.rybin/batch_results/` | `/Users/d.rybin/batch_results2/` |
| Вселенная | CMC ~top-50 → Binance spot USDT | CMC #50–200 → Binance spot USDT, без пересечения с top-50 |
| Список | `scripts/symbols_top50.txt` | `scripts/symbols_cmc50_200.txt` |
| Успешно | **49 / 50** (нет HYPEUSDT — fetch fail) | **73 / 73** |
| Алгоритмов | 21 | 21 |
| Периоды | `full`, `last_2_years`, `current_year` | то же |
| Окно `current_year` | календарный год последней свечи (~2026 YTD) | то же |
| Важно про `full` / `last_2_years` | CSV скачан ~за 2 года → `full` ≈ `last_2_years` | у части mid-cap история короче 2 лет (листинг 2025–2026) |

Метрики ниже: для каждой пары (algo × symbol) берётся **лучший** результат sweep (`best.profit_pct`).  
**Median / mean** — по символам внутри алгоритма.  
**Win-rate** — доля символов с `profit_pct > 0`.  
**Bench** — семейства `rise` / `drop` / `trend` (широкий параметр-sweep → сильный overfitting).  
**Structure** — остальные (NR7, fib-trend, VCB, BR, LSL, CRT, …).

---

## 1. Главный вывод

1. **Ранг structure-алгоритмов стабилен** на majors (top-50) и mid-caps (CMC #50–200):  
   **fib-pullback-trend-v1 → nr7-trend-breakout-v1 → volatility-compression-breakout-v1 → breakout-retest-long-v2**, дальше около нуля / минус.
2. **Edge не специфичен только для top-50** — на mid-cap те же лидеры и аутсайдеры, medians почти совпали.
3. **Bench (rise/drop/trend) нельзя читать как live edge** — огромные % из best-of-sweep на том же ряду.
4. **breakout-retest-long (v1) сломан** (win ≈ 0%, сотни–тысячи сделок). **v2** чинит это.
5. **Liquidity Sweep Long v1–v5 и CRT** в обоих батчах в основном около нуля или в минусе — не кандидаты в «рабочий набор».
6. На **current_year** абсолюты сжаты ~в 3×, ранг structure сохраняется; **BR v2** становится хрупче (win падает с ~82% до ~63–66%).

### Практический рабочий набор

| Приоритет | Алгоритм | Комментарий |
|-----------|----------|-------------|
| 1 | `fib-pullback-trend-v1` | Лучший «честный» structure по median + win |
| 2 | `nr7-trend-breakout-v1` | Чуть ниже median, больше сделок |
| 3 | `volatility-compression-breakout-v1` | Высокий win, мало сделок |
| 4 | `breakout-retest-long-v2` | Плюс, но на CY win заметно падает |
| Не гонять | `breakout-retest-long` (v1), LSL v2/v4/v5 | Системно слабые |
| Бенчмарк only | `trend-long-sma*`, rise/drop/trend | Upper-bound / overfitting |

---

## 2. Batch 1 — top-50

### 2.1 Алгоритмы · last_2_years

| Алгоритм | Тип | Median % | Mean % | Win % | Med trades |
|----------|-----|----------|--------|-------|------------|
| drop | bench | +719.4 | +821.5 | 100 | 34 |
| trend-long-sma | structure* | +227.2 | +340.8 | 100 | 8 |
| trend-long-sma-retest | structure* | +195.0 | +235.8 | 100 | 5 |
| trend | bench | +192.4 | +247.6 | 100 | 12 |
| trend-long | structure* | +184.7 | +253.2 | 100 | 6 |
| rise-2d-profit | bench | +162.7 | +289.9 | 100 | 16 |
| rise | bench | +145.3 | +262.6 | 100 | 7 |
| drop-margin | bench | +97.5 | +88.8 | 95.9 | 41 |
| **nr7-trend-breakout-v1** | structure | **+47.1** | +56.7 | **91.8** | 206 |
| **fib-pullback-trend-v1** | structure | **+43.4** | +54.7 | **98.0** | 17 |
| **volatility-compression-breakout-v1** | structure | **+32.3** | +35.9 | **100** | 12 |
| **breakout-retest-long-v2** | structure | **+13.0** | +13.0 | **81.6** | 165 |
| fib-pullback-long-v2 | structure | +0.9 | +1.9 | 53.1 | 1 |
| liquidity-sweep-long-v3 | structure | −0.8 | −1.3 | 44.9 | 24 |
| liquidity-sweep-long | structure | −1.7 | +5.2 | 49.0 | 33 |
| fib-pullback-long | structure | −4.2 | −5.2 | 44.9 | 18 |
| crt-long | structure | −7.5 | −2.3 | 38.8 | 56 |
| liquidity-sweep-long-v5 | structure | −11.4 | −8.8 | 28.6 | 18 |
| liquidity-sweep-long-v2 | structure | −11.6 | −8.8 | 36.7 | 94 |
| liquidity-sweep-long-v4 | structure | −14.6 | −3.8 | 30.6 | 91 |
| **breakout-retest-long** | structure | **−63.6** | −61.9 | **0.0** | 1112 |

\*SMA/trend-long — формально structure, но с широким sweep → ближе к бенчмарку по интерпретации.

### 2.2 Алгоритмы · current_year

| Алгоритм | Median % | Win % | vs L2Y (pp) |
|----------|----------|-------|-------------|
| drop | +127.2 | 100 | −592 |
| trend-long-sma-retest | +51.1 | 100 | −144 |
| trend-long-sma | +49.5 | 100 | −178 |
| fib-pullback-trend-v1 | **+13.4** | 98 | −30 |
| nr7-trend-breakout-v1 | **+9.4** | 84 | −38 |
| volatility-compression-breakout-v1 | **+7.4** | 100 | −25 |
| breakout-retest-long-v2 | **+2.8** | **63** | −10 (win −19) |
| LSL / CRT / fib-v1 | −1…−10 | 25–47 | около нуля |
| breakout-retest-long | −24.8 | 0 | +39 (всё ещё мёртв) |

### 2.3 Монеты · last_2_years (median across all algos)

**Топ:** MANA (+85%), WLD (+76%), AXS (+67%), PEPE (+61%), FET (+58%).  
**Стабильность:** BTC не топ по median (+21%), но win алгосов в плюсе высокий (~95%).  
**Хвост:** OP (+12%), APT (+8%), FIL (+4%).

На CY топ сменился: WLD, INJ, NEAR, FET, TIA; majors относительно лучше (ETH +14%, SOL +10%, BTC +7%).  
2y-чемпионы (MANA/AXS/PEPE) на CY резко просели.

### 2.4 Наблюдения batch 1

- HYPEUSDT не попал (нет / проблемы Binance spot на момент fetch).
- Выбросы вроде ZEC + rise-2d **+4286%** — артефакт sweep.
- В топ-3 по символам чаще всего drop / trend-long-sma — из-за ширины sweep, не из-за live edge.

---

## 3. Batch 2 — CMC #50–200

### 3.1 Алгоритмы · last_2_years

| Алгоритм | Тип | Median % | Mean % | Win % | Med trades |
|----------|-----|----------|--------|-------|------------|
| drop | bench | +740.8 | +1079.6 | 100 | 40 |
| trend-long-sma-retest | structure* | +235.2 | +309.7 | 100 | 7 |
| trend-long-sma | structure* | +234.2 | +317.1 | 98.6 | 12 |
| rise-2d-profit | bench | +213.1 | +305.4 | 100 | 18 |
| rise | bench | +207.1 | +264.9 | 100 | 10 |
| trend | bench | +203.8 | +361.1 | 98.6 | 19 |
| trend-long | structure* | +160.6 | +228.9 | 97.3 | 6 |
| drop-margin | bench | +81.7 | +81.8 | 91.8 | 57 |
| **fib-pullback-trend-v1** | structure | **+43.6** | +68.5 | **91.8** | 23 |
| **nr7-trend-breakout-v1** | structure | **+39.2** | +53.4 | **90.4** | 186 |
| **volatility-compression-breakout-v1** | structure | **+27.2** | +34.6 | **98.6** | 10 |
| **breakout-retest-long-v2** | structure | **+13.1** | +14.8 | **82.2** | 149 |
| fib-pullback-long-v2 | structure | +0.3 | +1.5 | 52.1 | 2 |
| liquidity-sweep-long | structure | −3.0 | +0.7 | 45.2 | 31 |
| fib-pullback-long | structure | −4.8 | −3.9 | 39.7 | 18 |
| liquidity-sweep-long-v3 | structure | −5.2 | −4.1 | 38.4 | 20 |
| liquidity-sweep-long-v2 | structure | −5.6 | +2.1 | 46.6 | 88 |
| crt-long | structure | −7.5 | +1.3 | 32.9 | 54 |
| liquidity-sweep-long-v4 | structure | −8.2 | +4.8 | 39.7 | 87 |
| liquidity-sweep-long-v5 | structure | −8.2 | −1.9 | 35.6 | 15 |
| **breakout-retest-long** | structure | **−61.9** | −56.6 | **0.0** | 986 |

### 3.2 Алгоритмы · current_year

| Алгоритм | Median % | Win % |
|----------|----------|-------|
| fib-pullback-trend-v1 | **+16.8** | 94.5 |
| nr7-trend-breakout-v1 | **+13.8** | 86.3 |
| volatility-compression-breakout-v1 | **+11.4** | 98.6 |
| breakout-retest-long-v2 | **+3.5** | **65.8** |
| LSL / CRT / fib | −0…−4 | 27–48 |
| breakout-retest-long | −26.0 | 2.7 |

### 3.3 Монеты · last_2_years

**Топ (all-algos median):** DEXE (+148%), BANANAS31 (+77%), KAITO (+70%), PENGU (+67%), ZEN (+66%).  
**Самый стабильный win:** KAITO (~95% алгосов в плюсе).  
**Хвост:** AERO (0%, win 20%), GENIUS (+2.5%), NIGHT (+6%) — короткие листинги.

Структура истории (candle_from): ~51 монета с 2024, ~16 с 2025, ~6 с 2026.  
Примеры: AERO с 2026-07, GENIUS с 2026-05, MORPHO с 2025-10. Для них «last_2_years» = available history.

### 3.4 Наблюдения batch 2

- Failed пустой; AERO без `rise` (некритично).
- Drop-выбросы ещё жёстче (EIGEN drop +8726%, DEXE trend +6826%) — волатильные mid-caps + sweep.
- Без бенчмарков structure-median у части «топов all-algos» (JUP, CFX, RSR) уже **отрицательный** — топ по all-algos часто раздут rise/drop.

---

## 4. Сравнение batch 1 vs batch 2

### 4.1 Structure medians · last_2_years

| Алгоритм | Top-50 | CMC #50–200 | Δ (pp) |
|----------|--------|-------------|--------|
| fib-pullback-trend-v1 | +43.4 | +43.6 | **+0.2** |
| nr7-trend-breakout-v1 | +47.1 | +39.2 | −7.9 |
| volatility-compression-breakout-v1 | +32.3 | +27.2 | −5.1 |
| breakout-retest-long-v2 | +13.0 | +13.1 | **+0.1** |
| fib-pullback-long-v2 | +0.9 | +0.3 | −0.6 |
| crt-long | −7.5 | −7.5 | **0.0** |
| liquidity-sweep-long-v2 | −11.6 | −5.6 | +6.0 |
| liquidity-sweep-long-v4 | −14.6 | −8.2 | +6.4 |
| breakout-retest-long | −63.6 | −61.9 | +1.7 |

**Интерпретация:** гипотезы fib-trend / BR v2 / CRT воспроизводятся почти 1:1. NR7 и VCB чуть слабее на mid-cap (больше шума), но остаются сильными лидерами. LSL на mid-cap чуть менее убыточен, но всё ещё не «в плюсе».

### 4.2 Current year · structure

| Алгоритм | Top-50 CY | CMC CY |
|----------|-----------|--------|
| fib-pullback-trend-v1 | +13.4 | +16.8 |
| nr7 | +9.4 | +13.8 |
| vcb | +7.4 | +11.4 |
| BR v2 | +2.8 (win 63%) | +3.5 (win 66%) |
| BR v1 | −24.8 (win 0%) | −26.0 (win 2.7%) |

На коротком окне mid-cap даже чуть выше по fib/nr7/vcb median — возможно из-за более волатильных пар и меньшей длины ряда.

### 4.3 Монеты

| | Top-50 | CMC #50–200 |
|--|--------|-------------|
| Лидеры L2Y | MANA, WLD, AXS, PEPE, FET | DEXE, BANANAS31, KAITO, PENGU, ZEN |
| Лидеры CY | WLD, INJ, NEAR, FET, TIA | DEXE, KAITO, BANANAS31, DCR, ZAMA |
| Стабильность | BTC (высокий win, скромный median) | KAITO (высокий win) |
| Хвост | FIL, APT, OP; на CY — ETC/DOT/GRT flat | AERO, GENIUS, NIGHT (короткий listing) |
| Перенос топа | 2y-альты не держатся на CY | то же + осторожность с новыми листингами |

### 4.4 Что совпало / что нет

| Совпало | Не совпало / нюанс |
|---------|-------------------|
| Порядок structure-лидеров | Топ монет полностью другой (разные вселенные) |
| BR v1 мёртв | NR7/VCB чуть слабее на mid-cap L2Y |
| LSL/CRT слабые | У mid-cap больше sweep-выбросов и коротких историй |
| BR v2 хрупче на CY | Абсолюты drop/rise ещё раздутее на mid-cap |
| Bench раздут на обоих | `full` ≈ `last_2_years` только если CSV реально ~2y |

---

## 5. Рекомендации

1. **Фокус алгосов:** `fib-pullback-trend-v1`, `nr7-trend-breakout-v1`, `volatility-compression-breakout-v1`; `breakout-retest-long-v2` — с оглядкой на CY win-rate.
2. **Не гонять по умолчанию:** `breakout-retest-long` (v1), семейство LSL (особенно v2/v4/v5), `crt-long` как основной.
3. **Интерпретация отчётов:** всегда отделять bench от structure; смотреть median + win, не mean и не единичные outliers.
4. **Периоды:** `current_year` — обязательная проверка; `last_2_years` для новых листингов = available history.
5. **Монеты:** не переносить топ all-algos median в live без фильтра по ликвидности и длине истории; для mid-cap отдельно смотреть structure-only median.
6. **Дальше (опционально):** walk-forward / out-of-sample без полного sweep; урезанный `ALGOS` в следующих батчах; исключать пары с listing &lt; N месяцев.

---

## 6. Артефакты и canvas

| Артефакт | Путь |
|----------|------|
| Batch 1 raw | `/Users/d.rybin/batch_results/` |
| Batch 2 raw | `/Users/d.rybin/batch_results2/` |
| Canvas top-50 (2y) | `canvases/Batch-Top50-Results.canvas.tsx` |
| Canvas top-50 (CY) | `canvases/Batch-Current-Year.canvas.tsx` |
| Canvas CMC #50–200 | `canvases/Batch-CMC50-200-Results.canvas.tsx` |
| Списки символов | `scripts/symbols_top50.txt`, `scripts/symbols_cmc50_200.txt` |
| Запуск batch | `scripts/batch_top50_vps.sh`, `scripts/README.md` |

Дата сводки: 2026-08-05. Цифры — агрегаты best-of-sweep; это не гарантия live PnL.
