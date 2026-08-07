package lineth.vertx

import io.vertx.core.http.HttpClientOptions
import lineth.getPortWithSchemeDefaults
import java.net.URI

fun <T : HttpClientOptions> T.setDefaultsFrom(uri: URI): T {
  isSsl = uri.scheme.lowercase() == "https"
  defaultHost = uri.host
  defaultPort = uri.getPortWithSchemeDefaults()

  return this
}
