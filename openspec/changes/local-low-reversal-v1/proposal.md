# Local Low Reversal v1

Test a Spot long-only reversal: a completed 1h candle must make a strict low below the prior twenty 1h lows. The signal fills no earlier than the next 1h open. Stop is the local low; exits are half at 1R, the remainder at 2R, or 48 hours. No short leg, parameter sweep, or holdout access.
