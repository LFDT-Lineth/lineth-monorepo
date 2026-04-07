import de.undercouch.gradle.tasks.download.DownloadAction
import org.gradle.api.Action
import org.gradle.api.DefaultTask
import org.gradle.api.Plugin
import org.gradle.api.Project
import org.gradle.api.file.DirectoryProperty
import org.gradle.api.file.DuplicatesStrategy
import org.gradle.api.model.ObjectFactory
import org.gradle.api.plugins.JavaPlugin
import org.gradle.api.provider.ListProperty
import org.gradle.api.tasks.CacheableTask
import org.gradle.api.tasks.Input
import org.gradle.api.tasks.Internal
import org.gradle.api.tasks.OutputDirectory
import org.gradle.api.tasks.TaskAction

import javax.inject.Inject
import java.nio.file.FileAlreadyExistsException
import java.nio.file.Files
import java.nio.file.Path
import java.time.Duration
import java.time.Instant

/** DSL object used only in the extension / build script. Not a task property. */
class FetchLibsSpec {
  final ListProperty<String> urls
  final ListProperty<String> libs

  /** Convenience setter: {@code url = "..."} is sugar for {@code urls = ["..."]}. */
  void setUrl(String url) {
    urls.add(url)
  }

  @Inject
  FetchLibsSpec(ObjectFactory objects) {
    urls = objects.listProperty(String)
    libs = objects.listProperty(String)
  }
}

@CacheableTask
abstract class DownloadNativeLibsTask extends DefaultTask {
  private static final Map<String, String> ARCH_RESOURCE_DIR_MAPPING = [
    "darwin_arm64" : "darwin-aarch64",
    "darwin_x86_64": "darwin-x86-64",
    "linux_arm64"  : "linux-aarch64",
    "linux_amd64"  : "linux-x86-64",
    "linux_x86_64" : "linux-x86-64"
  ]

  /**
   * Parallel lists: urls[i] is the zip URL, libsPerUrl[i] is a comma-separated
   * list of lib names to extract from that zip. Each fetchLibs block that declares
   * N urls produces N entries here — one per URL. Using flat @Input ListProperty<String>
   * rather than @Nested objects guarantees reliable Gradle cache fingerprinting.
   */
  @Input
  abstract ListProperty<String> getUrls()

  @Input
  abstract ListProperty<String> getLibsPerUrl()

  /**
   * Directory where downloaded zips are cached across task runs.
   * @Internal: the URL covers change detection; this dir is a performance
   * optimisation only and does not affect the declared task output.
   */
  @Internal
  abstract DirectoryProperty getDownloadCacheDir()

  @OutputDirectory
  abstract DirectoryProperty getResourcesDir()

  @TaskAction
  void execute() {
    List<String> urlList = urls.get()
    List<String> libsList = libsPerUrl.get()
    if (urlList.size() != libsList.size()) {
      throw new org.gradle.api.GradleException("urls and libsPerUrl must have the same number of entries")
    }

    Path cacheDir = downloadCacheDir.get().asFile.toPath()
    Path resDir = resourcesDir.get().asFile.toPath()

    [urlList, libsList].transpose().each { String url, String libsCsv ->
      List<String> libs = libsCsv.split(",").toList()
      Path zipPath = downloadZipIfAbsent(url, cacheDir)
      Path unzipDir = cacheDir.resolve(zipPath.fileName.toString().replaceFirst(/\.zip$/, ""))
      libs.each { String libName ->
        extractLibToResources(zipPath, unzipDir, libName, resDir)
      }
    }
  }

  protected Path downloadZipIfAbsent(String url, Path cacheDir) {
    String fileName = url.split("/").last()
    Path dest = cacheDir.resolve(fileName)

    if (Files.exists(dest)) {
      logger.lifecycle("Skipping download, zip already cached at {}", dest)
      return dest
    }

    try {
      Files.createDirectories(cacheDir)
    } catch (FileAlreadyExistsException ignored) {}

    logger.lifecycle("Downloading {}", url)
    DownloadAction action = new DownloadAction(project, this)
    action.src(url)
    action.dest(dest.toFile())
    action.overwrite(false)
    action.execute().get()

    return dest
  }

  protected void extractLibToResources(Path zipFile, Path unzipDir, String libName, Path resourcesDir) {
    unzipWithRetry(zipFile, unzipDir, Duration.ofSeconds(60))

    project.fileTree(unzipDir.toFile())
      .filter { File f -> f.name.contains(libName) && (f.name.endsWith(".so") || f.name.endsWith(".dylib")) }
      .each { File file ->
        String archDir = resolveArchDir(file.name)
        String destFileName = buildDestFileName(file.name, libName)
        Path destDir = resourcesDir.resolve(archDir)

        Files.createDirectories(destDir)
        logger.lifecycle("Copying {} → {}/{}", file.name, archDir, destFileName)
        project.copy {
          from file
          into destDir
          rename { destFileName }
        }
      }
  }

  protected static String resolveArchDir(String fileName) {
    Map.Entry<String, String> entry = ARCH_RESOURCE_DIR_MAPPING.find { key, _ -> fileName.contains(key) }
    if (entry == null) {
      throw new org.gradle.api.GradleException("No architecture mapping found for: ${fileName}")
    }
    return entry.value
  }

  protected static String buildDestFileName(String fileName, String libName) {
    def matcher = (fileName =~ /v\d+\.\d+\.\d+/)
    String version = matcher.find() ? matcher.group(0) : "unknown"
    String ext = fileName.split("\\.").last()
    return "lib${libName}_jna_${version}.${ext}"
  }

  protected void unzipWithRetry(Path zipFile, Path outputDir, Duration timeout) {
    if (outputDir.toFile().exists()) {
      logger.lifecycle("Skipping unzip, already extracted: {}", outputDir)
      return
    }

    boolean unzipped = false
    Instant start = Instant.now()
    Throwable lastError = null

    while (!unzipped && Duration.between(start, Instant.now()).compareTo(timeout) < 0) {
      try {
        lastError = null
        project.copy {
          from project.zipTree(zipFile)
          into outputDir
        }
        unzipped = true
      } catch (Exception e) {
        lastError = e
        Thread.sleep(1000)
      }
    }

    if (lastError != null) {
      throw new org.gradle.api.GradleException("Failed to unzip ${zipFile}", lastError)
    }
  }
}

class DownloadNativeLibsExtension {
  private final List<FetchLibsSpec> specs = []
  private final ObjectFactory objects

  @Inject
  DownloadNativeLibsExtension(ObjectFactory objects) {
    this.objects = objects
  }

  void fetchLibs(Action<FetchLibsSpec> action) {
    FetchLibsSpec spec = objects.newInstance(FetchLibsSpec)
    action.execute(spec)
    specs << spec
  }

  List<FetchLibsSpec> getSpecs() {
    Collections.unmodifiableList(specs)
  }
}

class DownloadNativeLibsPlugin implements Plugin<Project> {
  static final String TASK_NAME = "downloadNativeLibs"

  @Override
  void apply(Project project) {
    DownloadNativeLibsExtension extension = project.extensions.create("downloadNativeLibs", DownloadNativeLibsExtension)

    DirectoryProperty cacheDir = project.objects.directoryProperty().convention(
      project.parent != null
        ? project.parent.layout.buildDirectory.dir("native-libs-cache")
        : project.layout.buildDirectory.dir("native-libs-cache")
    )

    def downloadTask = project.tasks.register(TASK_NAME, DownloadNativeLibsTask) {
      group = "Native Libs"
      description = "Downloads and extracts native library binaries into the build directory."
      downloadCacheDir.set(cacheDir)
      resourcesDir.set(project.layout.buildDirectory.dir("generated/native-libs/src/main/resources"))
      // Both providers are resolved lazily, after the build script's fetchLibs blocks have run.
      // Each spec may declare multiple URLs; flatten so urls[i] <-> libsPerUrl[i].
      urls.set(project.provider {
        extension.specs.collectMany { spec -> spec.urls.get() }
      })
      libsPerUrl.set(project.provider {
        extension.specs.collectMany { spec ->
          String libsCsv = spec.libs.get().join(",")
          spec.urls.get().collect { libsCsv }
        }
      })
    }

    // Add the task output to the main source set's resources so files are packaged
    // into the jar. The Provider-backed srcDir carries the task dependency implicitly.
    // EXCLUDE duplicates: src/main/resources may contain leftover dylibs from a
    // previous run of the old script plugin; the generated output takes precedence.
    project.plugins.withType(JavaPlugin) {
      project.sourceSets.main.resources.srcDir(downloadTask.flatMap { it.resourcesDir })
      project.tasks.named("processResources").configure {
        duplicatesStrategy = DuplicatesStrategy.EXCLUDE
      }
    }

    // Ensure native libs are present before compilation.
    project.plugins.withId("org.jetbrains.kotlin.jvm") {
      project.tasks.named("compileKotlin").configure { dependsOn downloadTask }
    }
    project.plugins.withId("java") {
      project.tasks.named("compileJava").configure { dependsOn downloadTask }
    }
  }
}
