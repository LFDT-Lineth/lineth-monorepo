package linea.log4j

import org.apache.logging.log4j.Level
import org.apache.logging.log4j.core.config.Configurator

fun configureLoggers(
  rootLevel: Level = Level.INFO,
  vararg loggerConfigs: Pair<String, Level>
) {
  Configurator.setRootLevel(rootLevel)
  loggerConfigs.forEach { (loggerName, level) ->
    Configurator.setLevel(loggerName, level)
  }
}
