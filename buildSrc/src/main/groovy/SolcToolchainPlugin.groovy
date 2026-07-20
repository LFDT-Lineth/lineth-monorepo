import org.gradle.api.GradleException
import org.gradle.api.Plugin
import org.gradle.api.Project

class SolcToolchainExtension {
  /** Pinned solc version this project's Solidity/Yul compilation should use. */
  String version
}

/**
 * org.web3j.solidity (and this repo's linea.yul-plugin, which calls org.web3j.sokt directly)
 * resolve solc through web3j-sokt, which parses an upstream release index with a strict JSON
 * reader that rejects keys its model does not know. A newer index added such a key, breaking
 * every solc resolution, and there is no sokt release that tolerates it. This plugin sidesteps
 * the resolver entirely: it downloads the pinned solc binary straight from GitHub releases and
 * wires it into the `solidity` / `yul` extensions' `executable` option instead of resolving by
 * version.
 */
class SolcToolchainPlugin implements Plugin<Project> {
  private static final List<String> WIRED_TASK_NAMES = [
    'compileSolidity',
    'compileTestSolidity',
    'compileYul',
    'compileTestYul'
  ]

  @Override
  void apply(Project project) {
    SolcToolchainExtension extension = project.extensions.create('solcToolchain', SolcToolchainExtension)

    def executableFile = { ->
      String version = extension.version
      if (version == null) {
        throw new GradleException("solcToolchain { version = '...' } must be set")
      }
      String suffix = System.getProperty('os.name').toLowerCase().contains('win') ? '.exe' : ''
      new File(project.layout.buildDirectory.get().asFile, "solc/solc-${version}${suffix}")
    }

    def downloadTask = project.tasks.register('downloadSolc') {
      group = 'build setup'
      description = 'Downloads the pinned solc binary, bypassing the broken web3j-sokt release index.'
      outputs.file(project.provider(executableFile))
      onlyIf { !executableFile().exists() }
      doLast {
        File target = executableFile()
        target.parentFile.mkdirs()
        String os = System.getProperty('os.name').toLowerCase()
        String asset = os.contains('win') ? 'solc-windows.exe' : os.contains('mac') ? 'solc-macos' : 'solc-static-linux'
        String url = "https://github.com/ethereum/solidity/releases/download/v${extension.version}/${asset}"
        logger.lifecycle("Downloading solc ${extension.version} from ${url}")
        target.withOutputStream { out ->
          new URL(url).withInputStream { input ->
            out << input
          }
        }
        target.setExecutable(true)
      }
    }

    // Wire the downloaded executable into whichever of org.web3j.solidity's `solidity` extension
    // and linea.yul-plugin's `yul` extension are present, once the build script has configured
    // them (e.g. `solidity { version = ... }`), and make the relevant compile tasks depend on the
    // download. Dynamic property access (no compile-time dependency on either plugin's classes).
    project.afterEvaluate {
      def solidityExtension = project.extensions.findByName('solidity')
      if (solidityExtension != null) {
        solidityExtension.executable = executableFile().absolutePath
      }
      def yulExtension = project.extensions.findByName('yul')
      if (yulExtension != null) {
        yulExtension.executable = executableFile().absolutePath
      }
      project.tasks.matching { WIRED_TASK_NAMES.contains(it.name) }.configureEach {
        dependsOn downloadTask
      }
    }
  }
}
