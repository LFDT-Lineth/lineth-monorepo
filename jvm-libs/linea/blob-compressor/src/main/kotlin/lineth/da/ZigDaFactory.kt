package lineth.da

import linea.jvm.ResourcesUtil.copyResourceToTmpDir
import java.nio.file.Files
import java.nio.file.Path
import java.util.concurrent.ConcurrentHashMap

/**
 * [DaCompressor]s and [DaStream]s over the native `lineth_da` library: [bundled] serves the libraries this jar ships,
 * [forLibrary] one library a path names.
 */
class ZigDaFactory private constructor(
  private val libraryOf: (DaCodecVersion) -> LinethDaLibrary,
) : DaCompressorFactory, DaStreamFactory {

  override fun create(version: DaCodecVersion, dictionary: Path, chainId: ULong): DaCompressor =
    ZigDaCompressor(version, libraryOf(version), Files.readAllBytes(dictionary), chainId)

  override fun open(version: DaCodecVersion, initialState: DaStreamPosition): DaStream =
    ZigDaStream(version, libraryOf(version), initialState)

  companion object {
    private const val DICTIONARIES_DIR = "compressor-dictionaries"

    private val bundledLibraries = ConcurrentHashMap<DaCodecVersion, LinethDaLibrary>()

    /** The libraries bundled in this jar, each extracted and loaded once. */
    val bundled = ZigDaFactory { version ->
      bundledLibraries.computeIfAbsent(version) { LinethDaLibrary.load(extractBundledLibrary(it)) }
    }

    /** Serves [DaCodecVersion.DEV] from the library at [path], loaded on first use. */
    fun forLibrary(path: Path): ZigDaFactory {
      val library by lazy { LinethDaLibrary.load(path) }
      return ZigDaFactory { version ->
        if (version != DaCodecVersion.DEV) {
          throw DaStreamException(
            DaStreamErrorType.INVALID_ARGUMENT,
            "the library $path serves ${DaCodecVersion.DEV}, not $version",
          )
        }
        library
      }
    }

    /** Extracts the dictionary `compressor-dictionaries/<name>.bin` of this jar to a temporary file. */
    fun bundledDictionary(name: String): Path =
      copyResourceToTmpDir("$DICTIONARIES_DIR/$name.bin", ZigDaFactory::class.java.classLoader)

    private fun extractBundledLibrary(version: DaCodecVersion): Path {
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
