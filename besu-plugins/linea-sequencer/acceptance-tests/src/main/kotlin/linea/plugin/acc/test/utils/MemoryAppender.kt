/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test.utils

import org.apache.logging.log4j.core.Appender
import org.apache.logging.log4j.core.Core
import org.apache.logging.log4j.core.Layout
import org.apache.logging.log4j.core.LogEvent
import org.apache.logging.log4j.core.appender.AbstractAppender
import org.apache.logging.log4j.core.config.Property
import org.apache.logging.log4j.core.config.plugins.Plugin
import org.apache.logging.log4j.core.config.plugins.PluginAttribute
import org.apache.logging.log4j.core.config.plugins.PluginElement
import org.apache.logging.log4j.core.config.plugins.PluginFactory
import java.io.ByteArrayOutputStream
import java.io.Serializable
import java.nio.charset.StandardCharsets

@Plugin(name = "Memory", category = Core.CATEGORY_NAME, elementType = Appender.ELEMENT_TYPE)
class MemoryAppender(name: String, layout: Layout<out Serializable>?) :
  AbstractAppender(name, null, layout, true, Property.EMPTY_ARRAY) {

  override fun append(event: LogEvent) {
    collectingOutput.write(layout.toByteArray(event))
  }

  companion object {
    private val collectingOutput = ByteArrayOutputStream()

    @JvmStatic
    @PluginFactory
    fun createAppender(
      @PluginAttribute("name") name: String,
      @PluginElement("Layout") layout: Layout<out Serializable>?,
    ): MemoryAppender {
      return MemoryAppender(name, layout)
    }

    @JvmStatic
    fun reset() {
      collectingOutput.reset()
    }

    @JvmStatic
    fun getLog(): String {
      return collectingOutput.toString(StandardCharsets.UTF_8)
    }
  }
}
