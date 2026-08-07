package lineth.logging

import java.net.URI

typealias LogFieldMask = (String) -> String

fun noopMask(field: String): String = field

fun maskEndpointPath(endpoint: String): String {
  val path = URI(endpoint).toURL().path
  return endpoint.replace(path, "/" + "*".repeat(path.length - 1))
}
