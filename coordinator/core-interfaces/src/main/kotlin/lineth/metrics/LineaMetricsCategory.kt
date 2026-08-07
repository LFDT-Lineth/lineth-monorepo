package lineth.metrics

import lineth.metrics.MetricsCategory

enum class LineaMetricsCategory : MetricsCategory {
  AGGREGATION,
  BATCH,
  BLOB,
  FORCED_TRANSACTION,
  CONFLATION,
  GAS_PRICE_CAP,
  L2_PRICING,
}
