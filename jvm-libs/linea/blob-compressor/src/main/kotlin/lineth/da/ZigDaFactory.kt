package lineth.da

import linea.jvm.ResourcesUtil.copyResourceToTmpDir
import java.nio.file.Path
import java.util.concurrent.ConcurrentHashMap

/**
 * [DaCompressor]s and [DaStream]s over the native `lineth_da` library: [bundled] serves the libraries this jar ships,
 * each with the dictionary of its version.
 */
class ZigDaFactory private constructor(
  private val libraryOf: (DaCodecVersion) -> LinethDaLibrary,
) : DaCompressorFactory, DaStreamFactory {

  override fun create(version: DaCodecVersion, chainId: ULong): DaCompressor =
    ZigDaCompressor(version, libraryOf(version), bundledDictionary(version), chainId)

  override fun open(version: DaCodecVersion, initialState: DaStreamPosition): DaStream =
    ZigDaStream(version, libraryOf(version), initialState)

  companion object {
    private const val DICTIONARIES_DIR = "compressor-dictionaries"

    private val bundledLibraries = ConcurrentHashMap<DaCodecVersion, LinethDaLibrary>()

    /** The libraries bundled in this jar, each extracted and loaded once. */
    val bundled = ZigDaFactory { version ->
      bundledLibraries.computeIfAbsent(version) { LinethDaLibrary.load(bundledLibrary(it)) }
    }

    /** The dictionary of [version]: `compressor-dictionaries/<name>.bin` of this jar. */
    internal fun bundledDictionary(version: DaCodecVersion): ByteArray {
      val name = version.dictionary
        ?: throw DaStreamException(DaStreamErrorType.INVALID_ARGUMENT, "$version has no dictionary")
      val resource = "$DICTIONARIES_DIR/$name.bin"
      return ZigDaFactory::class.java.classLoader.getResourceAsStream(resource)?.use { it.readAllBytes() }
        ?: throw DaStreamException(DaStreamErrorType.INVALID_ARGUMENT, "no bundled dictionary for $version: $resource")
    }

    internal fun bundledLibrary(version: DaCodecVersion): Path {
      val os = System.getProperty("os.name").lowercase()
      val arch = System.getProperty("os.arch").lowercase()
      val (osDir, extension) = when {
        "linux" in os -> "linux" to "so"
        "mac" in os || "darwin" in os -> "darwin" to "dylib"
        else -> null
      } ?: throw noBundledLibrary(version, "$os/$arch")
      val archDir = when (arch) {
        "amd64", "x86_64" -> "x86-64"
        "aarch64", "arm64" -> "aarch64"
        else -> throw noBundledLibrary(version, "$os/$arch")
      }
      val resource = "$osDir-$archDir/liblineth_da_jna_${version.version}.$extension"
      val classLoader = ZigDaFactory::class.java.classLoader
      if (classLoader.getResource(resource) == null) throw noBundledLibrary(version, resource)
      return copyResourceToTmpDir(resource, classLoader, "lineth-da-")
    }

    private fun noBundledLibrary(version: DaCodecVersion, where: String) =
      DaStreamException(DaStreamErrorType.INVALID_ARGUMENT, "no bundled library for $version: $where")
  }
}
