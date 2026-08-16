## ADDED Requirements

### Requirement: Causal signal execution
The engine SHALL execute a signal derived from a completed candle no earlier than the open of the next available candle and SHALL retain the signal time separately from the fill time.

#### Scenario: Close-confirmed entry
- **WHEN** a strategy confirms an entry using the close of candle T
- **THEN** the earliest eligible entry fill is the open of candle T+1

#### Scenario: Missing next interval
- **WHEN** the interval immediately after a signal is absent from the input data
- **THEN** the engine either fills at the next available candle under the manifest gap policy or rejects the signal with a recorded reason

### Requirement: Configurable trading costs
The engine SHALL apply the manifest commission and adverse slippage to every entry, partial exit, and final exit and SHALL report gross and net results.

#### Scenario: Long round trip under base costs
- **WHEN** a long position is opened and closed under the base profile
- **THEN** both fills include 10 basis points commission and 5 basis points adverse slippage

#### Scenario: Partial take profit
- **WHEN** half a position is closed at TP1 and the remainder closes later
- **THEN** commission and slippage are charged independently on both exit fills

### Requirement: Conservative stop and target fills
The engine SHALL model gaps and ambiguous intrabar paths without granting an unavailable ideal price.

#### Scenario: Gap through stop
- **WHEN** a long position's next candle opens below its stop
- **THEN** the exit reference price is that candle open and adverse sell slippage is applied

#### Scenario: Stop and target touched in one candle
- **WHEN** both stop and target lie within one candle range and no lower timeframe establishes ordering
- **THEN** the stop is processed first in the primary execution profile

#### Scenario: Strategy-timeframe execution
- **WHEN** a strategy declares a 15m or 1h timeframe and the canonical input is retained as 1m candles
- **THEN** the engine deterministically aggregates the source and evaluates fills, stops, targets, equity, and audit events on the declared strategy timeframe

### Requirement: Mark-to-market equity
The engine SHALL calculate equity on every evaluation bar from cash plus all open positions valued at current executable market prices.

#### Scenario: Open losing position at period end
- **WHEN** a test period ends with an open position below its entry
- **THEN** the loss is included in final equity, return, and drawdown

#### Scenario: Parameter ranking
- **WHEN** parameter candidates are compared
- **THEN** selection uses net mark-to-market metrics rather than realized cash alone

### Requirement: Risk-normalized position sizing
The standalone engine SHALL size positions from configured equity risk and stop distance and SHALL enforce cash and per-position notional limits.

#### Scenario: Valid stop distance
- **WHEN** current equity is 10,000, risk is 1%, and the entry-to-stop loss per unit is 5
- **THEN** the unconstrained quantity risks 100 before cost and is reduced if the notional cap is exceeded

#### Scenario: Invalid stop
- **WHEN** a strategy emits an entry without a finite positive stop distance
- **THEN** the engine rejects the entry and records `invalid_stop`

### Requirement: Deterministic simulation
Given identical candles, manifest, strategy version, and seed, the engine SHALL produce byte-equivalent normalized results.

#### Scenario: Repeated run
- **WHEN** the same experiment is executed twice without input changes
- **THEN** trades, equity curve, metrics, and decision output are identical

### Requirement: Execution audit trail
Every attempted signal SHALL be traceable through acceptance or rejection, fill calculation, costs, position changes, and exit reason.

#### Scenario: Sizing rejection
- **WHEN** an otherwise valid entry cannot satisfy standalone cash, stop, or notional rules
- **THEN** the report retains the signal with the rejection reason and applicable constraint values
