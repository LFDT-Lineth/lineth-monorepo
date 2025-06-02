package net.consensys.linea.logging

import org.apache.logging.log4j.Level
import org.apache.logging.log4j.core.Filter
import org.apache.logging.log4j.core.LogEvent
import org.apache.logging.log4j.core.config.plugins.Plugin
import org.apache.logging.log4j.core.config.plugins.PluginAttribute
import org.apache.logging.log4j.core.config.plugins.PluginFactory
import org.apache.logging.log4j.core.filter.AbstractFilter
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

data class LogCacheKey(
  val message: String,
  val logLevel: Level,
)

@Plugin(name = "DebouncingFilter", category = "Core", elementType = "filter")
class DebouncingFilter internal constructor(
  private val debounceTime: Duration = 30.seconds,
  private val maxCacheCapacity: Int = 1000,
) : AbstractFilter() {
  private val logTimesCache = run {
    // Source: https://stackoverflow.com/questions/15844035/best-hashmap-initial-capacity-while-indexing-a-list
    val expectedMapSize = ((maxCacheCapacity / 0.75) + 1).toInt()
    object : LinkedHashMap<LogCacheKey, Long>(expectedMapSize) {
      override fun removeEldestEntry(eldest: MutableMap.MutableEntry<LogCacheKey, Long>?): Boolean {
        return size > maxCacheCapacity
      }
    }
  }

  @Synchronized
  override fun filter(event: LogEvent): Filter.Result {
    val logToCache = LogCacheKey(event.message.formattedMessage, event.level)
    val lastLoggedAt = logTimesCache[logToCache]
    if (lastLoggedAt == null || (lastLoggedAt + debounceTime.inWholeMilliseconds < event.timeMillis)) {
      logTimesCache[logToCache] = event.timeMillis
      return Filter.Result.ACCEPT
    } else {
      return Filter.Result.DENY
    }
  }

  companion object {
    @PluginFactory
    @JvmStatic
    fun createFilter(
      @PluginAttribute(value = "debounceTimeMillis", defaultLong = 30000L) debounceTimeMillis: Long,
      @PluginAttribute(value = "maxCacheCapacity", defaultInt = 1000) maxCacheCapacity: Int,
    ): DebouncingFilter {
      return DebouncingFilter(
        debounceTime = debounceTimeMillis.milliseconds,
        maxCacheCapacity = maxCacheCapacity,
      )
    }
  }
}
