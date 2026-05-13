/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.metrics

import org.apache.logging.log4j.LogManager
import java.util.Optional
import net.consensys.linea.metrics.MetricsCategory as LineaMetricsCategory
import org.hyperledger.besu.plugin.services.metrics.MetricCategory as BesuMetricsCategory

class BesuMetricsCategoryAdapter private constructor(
  val category: LineaMetricsCategory,
) : BesuMetricsCategory {
  override fun getName(): String = category.name

  override fun getApplicationPrefix(): Optional<String> = Optional.of("maru")

  companion object {
    private val logger = LogManager.getLogger(BesuMetricsCategoryAdapter::class.java)

    fun from(category: LineaMetricsCategory): BesuMetricsCategory = BesuMetricsCategoryAdapter(category)

    fun toLineaMetricsCategory(besuMetricsCategory: BesuMetricsCategory): LineaMetricsCategory =
      try {
        MaruMetricsCategory.valueOf(besuMetricsCategory.name.uppercase())
      } catch (e: IllegalArgumentException) {
        logger.warn("Unknown BesuMetricsCategory: ${besuMetricsCategory.name}.", e)
        object : LineaMetricsCategory {
          override val name: String
            get() = besuMetricsCategory.name.uppercase()
        }
      }
  }
}
