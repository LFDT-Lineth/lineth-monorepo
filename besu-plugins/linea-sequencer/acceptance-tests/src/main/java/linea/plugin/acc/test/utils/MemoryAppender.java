/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test.utils;

import java.io.ByteArrayOutputStream;
import java.io.Serializable;
import java.nio.charset.StandardCharsets;
import lombok.SneakyThrows;
import org.apache.logging.log4j.core.Appender;
import org.apache.logging.log4j.core.Core;
import org.apache.logging.log4j.core.Layout;
import org.apache.logging.log4j.core.LogEvent;
import org.apache.logging.log4j.core.appender.AbstractAppender;
import org.apache.logging.log4j.core.config.Property;
import org.apache.logging.log4j.core.config.plugins.Plugin;
import org.apache.logging.log4j.core.config.plugins.PluginAttribute;
import org.apache.logging.log4j.core.config.plugins.PluginElement;
import org.apache.logging.log4j.core.config.plugins.PluginFactory;

@Plugin(name = "Memory", category = Core.CATEGORY_NAME, elementType = Appender.ELEMENT_TYPE)
public class MemoryAppender extends AbstractAppender {
  private static final ByteArrayOutputStream collectingOutput = new ByteArrayOutputStream();

  protected MemoryAppender(String name, Layout<? extends Serializable> layout) {
    super(name, null, layout, true, Property.EMPTY_ARRAY);
  }

  @PluginFactory
  public static MemoryAppender createAppender(
      @PluginAttribute("name") String name,
      @PluginElement("Layout") Layout<? extends Serializable> layout) {
    return new MemoryAppender(name, layout);
  }

  @SneakyThrows
  @Override
  public void append(LogEvent event) {
    collectingOutput.write(getLayout().toByteArray(event));
  }

  public static void reset() {
    collectingOutput.reset();
  }

  public static String getLog() {
    return collectingOutput.toString(StandardCharsets.UTF_8);
  }
}
