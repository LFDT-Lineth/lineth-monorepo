/*
 * Copyright Consensys Software Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

import org.gradle.api.Plugin
import org.gradle.api.Project
import org.gradle.process.ExecOperations
import javax.inject.Inject

class LineaBesuPlugin implements Plugin<Project> {

  private final ExecOperations execOperations

  @Inject
  LineaBesuPlugin(ExecOperations execOperations) {
    this.execOperations = execOperations
  }

  @Override
  void apply(Project project) {
    project.tasks.register('checkoutAndResolveVersion') {
      group = 'Build'
      description = 'Clone/fetch besu-eth/besu at besuCommit and resolve besu version = latest release tag + "-" + 7-char commit'
      doLast {
        def besuCommit = project.rootProject.libs.versions.besuCommit.get()
        def rootDir = project.rootProject.layout.projectDirectory.asFile.absolutePath
        def outputStream = new ByteArrayOutputStream()
        execOperations.exec {
          workingDir = project.rootProject.layout.projectDirectory.asFile
          environment 'BESU_DIR', "${rootDir}/tmp/besu-eth"
          environment 'BESU_COMMIT', besuCommit
          environment 'VERSION_LABEL', ""
          commandLine 'bash', "${rootDir}/linea-besu/scripts/checkout-and-resolve-version.sh"
          standardOutput = outputStream
        }
        def resolvedBesuVer = outputStream.toString().trim().readLines().last()
        project.rootProject.ext.resolvedBesuVer = resolvedBesuVer
        project.logger.lifecycle("Resolved besu version: ${resolvedBesuVer}")
      }
    }

    project.tasks.register('checkoutAndBuildBesu') {
      group = 'Build'
      description = 'Build Besu at the resolved version (distTar publish/publishToMavenLocal)'

      def publishToMaven = project.hasProperty('publishToMaven') ? project.publishToMaven.toBoolean() : false
      def publishGradleTaskName = publishToMaven ? "publish" : "publishToMavenLocal"
      def skipDownloadBesuDist = project.hasProperty('skipDownloadBesuDist') ? project.skipDownloadBesuDist.toBoolean() : false

      dependsOn 'checkoutAndResolveVersion'

      doLast {
        def resolvedBesuVer = project.rootProject.ext.resolvedBesuVer
        def shouldSkip = false
        if (publishToMaven) {
          shouldSkip = isBesuAvailableInMaven(project, resolvedBesuVer) &&
              canDownloadBesuDistributionFromMaven(project, resolvedBesuVer)
        } else {
          if (isBesuAndDistributionAvailableInMavenLocal(project, resolvedBesuVer)) {
            shouldSkip = true
          } else if (isBesuAvailableInMaven(project, resolvedBesuVer) && 
              (skipDownloadBesuDist || downloadBesuDistributionFromMaven(project, resolvedBesuVer))) {
            if (skipDownloadBesuDist) {
              project.logger.lifecycle("Skipping download besu distribution from maven as skipDownloadBesuDist=${skipDownloadBesuDist}")
            }
            shouldSkip = true
          }
        }
        if (shouldSkip) {
          project.logger.lifecycle("Skipping checkoutAndBuildBesu: Besu ${resolvedBesuVer} already available")
          return
        }
        def rootDir = project.rootProject.layout.projectDirectory.asFile.absolutePath
        execOperations.exec {
          workingDir = project.rootProject.layout.projectDirectory.asFile
          environment 'BESU_DIR', "${rootDir}/tmp/besu-eth"
          environment 'RESOLVED_BESU_VERSION', resolvedBesuVer
          environment 'CLOUDSMITH_USER', project.hasProperty('cloudsmithUser') ? project.cloudsmithUser : ''
          environment 'CLOUDSMITH_API_KEY', project.hasProperty('cloudsmithApiKey') ? project.cloudsmithApiKey : ''
          commandLine 'bash', "${rootDir}/linea-besu/scripts/build-dist-and-publish.sh", publishGradleTaskName
        }
      }
    }

    project.tasks.register('buildAndUpdateBesuVersionInLibsVersions') {
      group = 'Build'
      description = 'Updates gradle/libs.versions.toml besu field to the locally-built besu version'
      dependsOn 'checkoutAndBuildBesu'
      doLast {
        def localBesuVersion = project.rootProject.ext.resolvedBesuVer
        def libsVersionsFile = project.rootProject.file('gradle/libs.versions.toml')
        def content = libsVersionsFile.text
        content = content.replaceFirst(/(?m)^besu\s*=\s*"[^"]*"/, "besu = \"${localBesuVersion}\"")
        libsVersionsFile.text = content
        project.logger.lifecycle("Updated gradle/libs.versions.toml: besu = \"${localBesuVersion}\"")
      }
    }
  }

  private static boolean isBesuAndDistributionAvailableInMavenLocal(Project project, String version) {
    if (!version) return false
    def mavenLocalBase = System.getProperty('maven.repo.local') ?: "${System.getProperty('user.home')}/.m2/repository"
    def mavenLocalPom = new File(mavenLocalBase, "org/hyperledger/besu/bom/${version}/bom-${version}.pom")
    def pomExists = mavenLocalPom.exists()
    if (!pomExists) {
      project.logger.lifecycle("isBesuAndDistributionAvailableInMavenLocal: besu:${version} is not in maven local")
      return false
    } else {
      project.logger.lifecycle("isBesuAndDistributionAvailableInMavenLocal: Besu ${version} was found in maven local")
    }
    def besuDistTar = project.rootProject.file("tmp/besu-eth/build/distributions/besu-${version}.tar.gz")
    def distExists = besuDistTar.exists()
    if (!distExists) {
      project.logger.lifecycle("isBesuAndDistributionAvailableInMavenLocal: besu-${version}.tar.gz distribution doesn't exist")
    } else {
      project.logger.lifecycle("isBesuAndDistributionAvailableInMavenLocal: besu-${version}.tar.gz distribution was found under \"tmp/besu-eth/build/distributions\"")
    }
    return pomExists && distExists
  }

  private static boolean isBesuAvailableInMaven(Project project, String version) {
    if (!version) return false
    def candidates = [
        "https://artifacts.consensys.net/public/linea-besu/maven/org/hyperledger/besu/bom/${version}/bom-${version}.pom",
        "https://repo.maven.apache.org/maven2/org/hyperledger/besu/bom/${version}/bom-${version}.pom",
        "https://hyperledger.jfrog.io/hyperledger/besu-maven/org/hyperledger/besu/bom/${version}/bom-${version}.pom",
    ]
    def connectTimeoutMs = 5000
    def readTimeoutMs = 5000
    for (def pomUrl : candidates) {
      def conn = null
      try {
        conn = (java.net.HttpURLConnection) new URL(pomUrl).openConnection()
        conn.setConnectTimeout(connectTimeoutMs)
        conn.setReadTimeout(readTimeoutMs)
        conn.setRequestMethod('HEAD')
        if (conn.getResponseCode() == 200) {
          project.logger.lifecycle("isBesuAvailableInMaven: Besu ${version} found at maven repo: ${pomUrl}")
          return true
        }
      } catch (Exception ignored) {
      } finally {
        if (conn != null) conn.disconnect()
      }
    }
    project.logger.lifecycle("isBesuAvailableInMaven: Besu ${version} not found in any maven repo")
    return false
  }

  private static boolean canDownloadBesuDistributionFromMaven(Project project, String version) {
    if (!version) return false
    def baseUrl = "https://artifacts.consensys.net/public/linea-besu/raw/names/linea-besu.tar.gz/versions/"
    def url = "${baseUrl}${version}/besu-${version}.tar.gz"
    def conn = null
    try {
      conn = (java.net.HttpURLConnection) new URL(url).openConnection()
      conn.setConnectTimeout(10000)
      conn.setReadTimeout(60000)
      conn.setRequestMethod('HEAD')
      if (conn.getResponseCode() == 200) {
        project.logger.lifecycle("canDownloadBesuDistributionFromMaven: Found besu distribution from Maven (${url})")
        return true
      }
      return false
    } finally {
      if (conn != null) conn.disconnect()
    }
  }

  private static boolean downloadBesuDistributionFromMaven(Project project, String version) {
    if (!version) return false
    def destDir = project.rootProject.file("tmp/besu-eth/build/distributions")
    def destFile = new File(destDir, "besu-${version}.tar.gz")
    if (destFile.exists()) {
      project.logger.lifecycle("downloadBesuDistributionFromMaven: Found existing besu distribution at ${destFile}, skipping download")
      return true
    }
    def baseUrl = "https://artifacts.consensys.net/public/linea-besu/raw/names/linea-besu.tar.gz/versions/"
    def url = "${baseUrl}${version}/besu-${version}.tar.gz"
    def conn = null
    try {
      conn = (java.net.HttpURLConnection) new URL(url).openConnection()
      conn.setConnectTimeout(10000)
      conn.setReadTimeout(60000)
      conn.setRequestMethod('GET')
      destDir.mkdirs()

      if (conn.getResponseCode() != 200) {
        project.logger.lifecycle("downloadBesuDistributionFromMaven: Could not find and download besu distribution from Maven (${url})")
        return false
      }
      conn.getInputStream().withStream { input ->
        destFile.withOutputStream { it << input }
      }
      project.logger.lifecycle("downloadBesuDistributionFromMaven: Downloaded besu-${version}.tar.gz from Maven to ${destFile}")
      return true
    } catch (Exception e) {
      project.logger.lifecycle("downloadBesuDistributionFromMaven: Failed to download besu distribution from Maven (${url}): ${e.message}")
      if (destFile.exists()) {
        destFile.delete()
        project.logger.lifecycle("downloadBesuDistributionFromMaven: Removed partial/corrupt file so next run can retry: ${destFile}")
      }
      return false
    } finally {
      if (conn != null) conn.disconnect()
    }
  }
}
