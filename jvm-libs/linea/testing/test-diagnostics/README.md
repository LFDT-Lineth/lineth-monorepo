# test-diagnostics

Evidence for tests that are timing-sensitive and flaky on some machines, mostly large CI runners. It is on the
test runtime classpath of every Kotlin project of the root build (`build.gradle`), and does nothing outside
Gradle: the listener is inert unless `lineth.test.diagnostics.dir` is set.

## What each test JVM writes

Under `<project>/build/test-diagnostics/<test task>/`:

| File | Content |
|------|---------|
| `gc-<pid>.log` | GC and safepoint log (`-Xlog:gc*,safepoint`, 2 files of 8 MB at most). Its first lines give the CPUs and memory the JVM sees (`gc,init`), later lines the pause times. |
| `thread-dump-<pid>-<n>.txt` | Written while a test is **still running** after 3 s, 30 s and 4 min (the 5 min JUnit timeout kills tests later). At most 20 per JVM and one per second: when many tests stall together, one dump lists them all. |

A thread dump starts with the tests running and for how long, then the figures that tell the usual causes apart:

- **CPU starvation**: `avgCpuCoresSinceStart` versus `availableProcessors`, `systemLoadAverage`, runnable threads with little CPU time.
- **GC pressure**: `heap used` close to `max`, GC time, and the pauses in `gc-<pid>.log`.
- **A lock or a deadlock**: `BLOCKED` threads, `waiting on ... owned by`, `DEADLOCK among threads`.
- **Something never completing**: every thread idle in its stack.

Then the 15 threads with the most CPU time, and every thread with its full stack.

The listener also prints one `[test-diagnostics]` line per JVM (processors, heap, GC) and one per dump to stderr.
Gradle keeps that in the `<system-err>` of the XML test report, not in its console output.

## In CI

`coordinator-testing` and `maru-testing` upload `**/build/test-diagnostics/**` as `test-diagnostics-coordinator` and
`test-diagnostics-maru` when they fail, and log the runner's CPUs and memory first (`log-runner-resources` action).

## Locally

```shell
# dump after 20 ms instead of 3 s / 30 s / 4 min, to see what a dump looks like
./gradlew :jvm-libs:linea:clients:linea-state-manager:test --rerun -PtestStallThresholdsMs=20
```

The files are in `jvm-libs/linea/clients/linea-state-manager/build/test-diagnostics/test/`. The arguments are passed
through an argument provider, so they are not task inputs: they do not invalidate test results or cache entries.
