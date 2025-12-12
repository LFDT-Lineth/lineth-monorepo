# Launch test coverage with local sonarqube server

## Development Setup

### Install docker

```
brew install docker
```

### Install docker compose plugin

```
brew install docker-compose
```

Add to your ~/.docker/config.json

```
 "cliPluginsExtraDirs": [
     "/opt/homebrew/lib/docker/cli-plugins"
 ]
```

## Produce Jacoco test reports

Sonarqube server needs a third party test coverage report to display a coverage score. This test coverage report is produced by Jacoco plugin.

### For one report

To produce a report, you can either

- Run tests tasks locally with

```
GOMEMLIMIT=26GiB ./gradlew test
GOMEMLIMIT=26GiB ./gradlew fastReplayTests
GOMEMLIMIT=26GiB ./gradlew nightlyTests
GOMEMLIMIT=26GiB ./gradlew weeklyTests
GOMEMLIMIT=26GiB ./gradlew -Dblockchain=Ethereum referenceBlockchainTests
GOMEMLIMIT=26GiB ./gradlew -Dblockchain=Ethereum referenceGeneralStateTests
```

These tasks are finalized by Jacoco test reports

- Or get xml report from the CI pipeline in tests actions

Paste the xml report in the following paths :

- for unit tests
  `/arithmetization/build/reports/jacoco/test/jacocoTestReport.xml`
- for fast replay tests
  `/arithmetization/build/reports/jacoco/jacocoFastReplayTestsReport/jacocoFastReplayTestsReport.xml`
- for nightly tests
  `/arithmetization/build/reports/jacoco/jacocoNightlyTestsReport/jacocoNightlyTestsReport.xml`
- for weekly tests
  `/arithmetization/build/reports/jacoco/jacocoWeeklyTestsReport/jacocoWeeklyTestsReport.xml`
- for reference blockchain tests
  `/referenceBlockchainTests/build/reports/jacoco/jacocoReferenceBlockchainTestsReport/jacocoReferenceBlockchainTestsReport.xml`
- for reference state tests
  `/referenceBlockchainTests/build/reports/jacoco/jacocoReferenceGeneralStateTestsReport/jacocoReferenceGeneralStateTestsReport.xml`

### Concatenate Jacoco reports

Get jacoco exec files from the CI pipeline in the uploaded artifacts tests actions

Paste the exec files in the following paths :

- for unit tests
  `/arithmetization/build/jacoco/test.exec`
- for fast replay tests
  `/arithmetization/build/jacoco/jacocoFastReplayTests.exec`
- for nightly tests
  `/arithmetization/build/jacoco/jacocoNightlyTests.exec`
- for weekly tests
  `/arithmetization/build/jacoco/jacocoWeeklyTests.exec`
- for reference blockchain tests
  `/reference-tests/build/jacoco/referenceBlockchainTests.exec`
- for reference state tests
  `/reference-tests/build/jacoco/referenceGeneralStateTests.exec`

Execute and concatenate the results with the `jacocoUnitFastReplayWeeklyNightlyReferenceBlockchainAndStateTestsReport` task

```
./gradlew jacocoUnitFastReplayWeeklyNightlyReferenceBlockchainAndStateTestsReport
```

Note: if an xml report already exists, delete it to generate it again, else the task will be marked as Skipped.

## Launch local Sonarqube server

Run `docker compose up`

Wait for following log

```
sonarqube-1   | 2025.01.23 16:40:21 INFO  app[][o.s.a.SchedulerImpl] SonarQube is operational
```

Change default password to not get prompted

```
curl -u admin:admin -X POST "http://localhost:80/api/users/change_password?login=admin&previousPassword=admin&password=Adminadmin123*"
```

Go to `localhost:80` in your browser and login with the credentials `admin/Adminadmin123*`

Generate a token in `My Account` > `Security` > `Generate Tokens` with no expiration date

Paste this token in `build.gradle` file in the `sonarqube` task's properties

```
    property "sonar.token", "your_token"
```

## Coverage score with Sonarqube

Launch `sonar` gradle task in verification group with the corresponding property depending on the coverage score you want to display

```
./gradlew sonar -Dtests=Unit
./gradlew sonar -Dtests=FastReplay
./gradlew sonar -Dtests=Nightly
./gradlew sonar -Dtests=Weekly
./gradlew sonar -Dtests=ReferenceBlockchain
./gradlew sonar -Dtests=ReferenceState
./gradlew sonar -Dtests=All
```

Go to `localhost:80/projects` in your browser and check the results :

- overall score
- line coverage (absolute and %)
- condition coverage (absolute and %)

## Exit Sonarqube server

```
docker compose down
```
