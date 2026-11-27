#!/usr/bin/env bash
# Collects and logs information about the runner executing the current job: hardware, kernel,
# container (cgroup) limits, Docker daemon, tool versions and, optionally, a short micro-benchmark.
#
# Run it twice in a job to also see how the job used the runner:
#
#   collect-runner-info.sh start   # specs + baseline counters (cgroup CPU/memory/PSI, host steal)
#   collect-runner-info.sh end     # deltas since `start`: CPU throttling, pressure, OOM kills, peak memory
#
# Output goes to collapsed groups in the job log, to the job summary, and to one greppable line:
#   RUNNER_INFO phase=<phase> key="value" key="value" ...
#
# Env:
#   RUNNER_INFO_BENCHMARK  "true" to run a ~10 s CPU and disk micro-benchmark (start phase only)
#
# Best effort by design: there is deliberately no `set -e`, every probe tolerates missing
# files/tools, and the script always exits 0 so a diagnostics step can never fail the job.
# Only allow-listed variables are printed; the environment is never dumped.

PHASE="${1:-start}"
case "$PHASE" in
  start | end) ;;
  *)
    echo "::warning::collect-runner-info: unknown phase '$PHASE' (expected start or end), nothing collected"
    exit 0
    ;;
esac

OS="$(uname -s 2>/dev/null)"
STATE_DIR="${RUNNER_TEMP:-${TMPDIR:-/tmp}}/runner-info"
mkdir -p "$STATE_DIR" 2>/dev/null
CURRENT="$STATE_DIR/$PHASE.kv"                 # values shown in the log, the summary and the one-liner
COUNTERS="$STATE_DIR/$PHASE.counters"          # raw counters, only used to compute start->end deltas
START_COUNTERS="$STATE_DIR/start.counters"
: >"$CURRENT"
: >"$COUNTERS"

CG=""      # cgroup directory of this job's container
CG_VER=""  # 1 or 2
NPROC=""
QUOTA=""   # CPU quota in CPUs, or "unlimited"
EFF=""     # min(nproc, quota)

# ---------------------------------------------------------------------------------------------
# helpers
# ---------------------------------------------------------------------------------------------
have() { command -v "$1" >/dev/null 2>&1; }
rd() { cat "$1" 2>/dev/null; } # prints a file, or nothing when it is missing/unreadable
is_linux() { [[ "$OS" == "Linux" ]]; }

group() { echo "::group::$*"; }
endgroup() { echo "::endgroup::"; }
run_group() {
  local title="$1"
  shift
  group "$title"
  "$@"
  endgroup
}

# kv <key> <value>: records a datapoint (log line + summary row + one-liner field). Never call it
# inside a command substitution, it prints.
kv() {
  local v
  v="$(printf '%s' "$2" | tr '\n\r' '  ' | sed -E 's/^ +| +$//g')"
  v="${v:-n/a}"
  printf '%s=%s\n' "$1" "$v" >>"$CURRENT"
  printf '  %-26s %s\n' "$1" "$v"
}

# ctr <name> <value>: records a raw counter for the start->end delta computation.
ctr() { [[ -n "$2" ]] && printf '%s=%s\n' "$1" "$2" >>"$COUNTERS"; }
get() { grep -m1 "^$2=" "$1" 2>/dev/null | cut -d= -f2-; }
# delta <name>: end - start for a counter; prints nothing unless both sides are known.
delta() {
  local a b
  a="$(get "$START_COUNTERS" "$1")"
  b="$(get "$COUNTERS" "$1")"
  if [[ "$a" =~ ^[0-9]+$ && "$b" =~ ^[0-9]+$ ]]; then echo $((b - a)); fi
}

pct() { awk -v a="$1" -v b="$2" 'BEGIN { if (a != "" && b != "" && b > 0) printf "%.1f", 100 * a / b }'; }
div() { awk -v a="$1" -v b="$2" 'BEGIN { if (a != "" && b != "" && b > 0) printf "%.2f", a / b }'; }
mib() { awk -v b="$1" 'BEGIN { if (b ~ /^[0-9]+$/) printf "%d", b / 1048576 }'; }
limit_mib() { awk -v b="$1" 'BEGIN { if (b == "max" || b + 0 >= 4e18) print "unlimited"; else if (b ~ /^[0-9]+$/) printf "%d", b / 1048576 }'; }
# psi_total <file> <some|full>: cumulative stall time in microseconds from a PSI file.
psi_total() { awk -v t="$2" '$1 == t { sub("total=", "", $5); print $5 }' "$1" 2>/dev/null; }
psi_avg10() { awk -v t="$2" '$1 == t { sub("avg10=", "", $2); print $2 }' "$1" 2>/dev/null; }

detect_cgroup() {
  is_linux || return 0
  if [[ "$(stat -fc %T /sys/fs/cgroup 2>/dev/null)" == "cgroup2fs" ]]; then
    CG_VER=2
    local p
    p="$(awk -F: '$1 == "0" { print $3 }' /proc/self/cgroup 2>/dev/null)"
    # Inside a cgroup namespace (the usual container setup) /sys/fs/cgroup already is the
    # container's cgroup. Without one, follow our own path so we never read the host root cgroup.
    if [[ -n "$p" && "$p" != "/" && -r "/sys/fs/cgroup$p/cpu.stat" ]]; then
      CG="/sys/fs/cgroup$p"
    else
      CG="/sys/fs/cgroup"
    fi
  elif [[ -d /sys/fs/cgroup/cpu ]]; then
    CG_VER=1
    CG="/sys/fs/cgroup"
  fi
}

# Prints the CPU quota in CPUs (e.g. "16.00"), "unlimited", or nothing when unknown.
cpu_quota() {
  if [[ "$CG_VER" == 2 ]]; then
    rd "$CG/cpu.max" | awk '{ if ($1 == "max" || $2 == 0) print "unlimited"; else printf "%.2f", $1 / $2 }'
  elif [[ "$CG_VER" == 1 ]]; then
    local q p
    q="$(rd "$CG/cpu/cpu.cfs_quota_us")"
    p="$(rd "$CG/cpu/cpu.cfs_period_us")"
    if [[ "$q" =~ ^-?[0-9]+$ && "$p" =~ ^[0-9]+$ && "$p" -gt 0 ]]; then
      awk -v q="$q" -v p="$p" 'BEGIN { if (q < 0) print "unlimited"; else printf "%.2f", q / p }'
    fi
  fi
}

# ---------------------------------------------------------------------------------------------
# start phase: specs
# ---------------------------------------------------------------------------------------------
collect_context() {
  kv repo "${GITHUB_REPOSITORY:-}"
  kv workflow "${GITHUB_WORKFLOW:-}"
  kv job "${GITHUB_JOB:-}"
  kv run_id_attempt "${GITHUB_RUN_ID:-}/${GITHUB_RUN_ATTEMPT:-}"
  kv sha "${GITHUB_SHA:0:10}"
  kv runner_name "${RUNNER_NAME:-}"
  kv runner_environment "${RUNNER_ENVIRONMENT:-}"
  kv runner_os_arch "${RUNNER_OS:-$OS}/${RUNNER_ARCH:-$(uname -m 2>/dev/null)}"
  kv runner_image "${ImageOS:-} ${ImageVersion:-}"
  kv runner_pod "${ACTIONS_RUNNER_POD_NAME:-}"
  kv hostname "$(hostname 2>/dev/null)"
  kv started_utc "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}

collect_os() {
  kv kernel "$(uname -srm 2>/dev/null)"
  local os_name
  os_name="$(grep -m1 '^PRETTY_NAME=' /etc/os-release 2>/dev/null | cut -d= -f2- | tr -d '"')"
  [[ -z "$os_name" ]] && os_name="$(sw_vers -productName 2>/dev/null) $(sw_vers -productVersion 2>/dev/null)"
  kv os "$os_name"
  kv machine "$(rd /sys/class/dmi/id/sys_vendor) $(rd /sys/class/dmi/id/product_name)"
  if [[ -n "${KUBERNETES_SERVICE_HOST:-}" ]]; then
    kv container kubernetes
  elif [[ -f /.dockerenv ]]; then
    kv container docker
  else
    kv container none
  fi
  # The kernel is shared with the other pods on the node, so these describe the node, not the pod.
  kv node_boot "$(uptime -s 2>/dev/null)"
  kv node_uptime "$(uptime -p 2>/dev/null)"
}

collect_cpu() {
  NPROC="$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null)"
  local model
  model="$(grep -m1 'model name' /proc/cpuinfo 2>/dev/null | cut -d: -f2)"
  [[ -z "$model" ]] && model="$(lscpu 2>/dev/null | awk -F: '/^Model name/ { print $2; exit }')"
  [[ -z "$model" ]] && model="$(sysctl -n machdep.cpu.brand_string 2>/dev/null)"
  [[ "${model// /}" == "-" ]] && model="" # some virtualised ARM CPUs report "-"
  kv cpu_model "$model"
  kv cpu_mhz "$(grep -m1 'cpu MHz' /proc/cpuinfo 2>/dev/null | cut -d: -f2)"
  kv cpus_node "$(grep -c '^processor' /proc/cpuinfo 2>/dev/null)"
  kv cpus_nproc "$NPROC"
  kv cpuset "$({ rd "$CG/cpuset.cpus.effective" || rd "$CG/cpuset/cpuset.cpus"; } 2>/dev/null)"
  QUOTA="$(cpu_quota)"
  kv cpu_quota_cores "$QUOTA"
  EFF="$(awk -v n="$NPROC" -v q="$QUOTA" 'BEGIN { if (q == "" || q == "unlimited" || q + 0 > n + 0) print n; else print q }')"
  kv cpus_effective "$EFF"
  kv cpu_weight "$({ rd "$CG/cpu.weight" || rd "$CG/cpu/cpu.shares"; } 2>/dev/null)"
  if have lscpu; then
    echo "--- lscpu"
    lscpu 2>/dev/null | grep -E '^(Architecture|CPU\(s\)|Thread|Core|Socket|NUMA node\(s\)|Model name|CPU max MHz|CPU min MHz|L2 cache|L3 cache|Hypervisor vendor|Virtualization type)'
  fi
}

collect_memory() {
  if [[ -r /proc/meminfo ]]; then
    kv mem_node_total_mib "$(awk '/^MemTotal/ { printf "%d", $2 / 1024 }' /proc/meminfo)"
    kv mem_node_avail_mib "$(awk '/^MemAvailable/ { printf "%d", $2 / 1024 }' /proc/meminfo)"
    kv swap_node_total_mib "$(awk '/^SwapTotal/ { printf "%d", $2 / 1024 }' /proc/meminfo)"
  elif have sysctl; then
    kv mem_total_mib "$(mib "$(sysctl -n hw.memsize 2>/dev/null)")"
  fi
  if [[ "$CG_VER" == 2 ]]; then
    kv mem_limit_mib "$(limit_mib "$(rd "$CG/memory.max")")"
    kv mem_high_mib "$(limit_mib "$(rd "$CG/memory.high")")"
    kv swap_limit_mib "$(limit_mib "$(rd "$CG/memory.swap.max")")"
    kv mem_current_mib "$(mib "$(rd "$CG/memory.current")")"
  elif [[ "$CG_VER" == 1 ]]; then
    kv mem_limit_mib "$(limit_mib "$(rd "$CG/memory/memory.limit_in_bytes")")"
    kv mem_current_mib "$(mib "$(rd "$CG/memory/memory.usage_in_bytes")")"
  fi
  if have free; then
    echo "--- free -m"
    free -m
  fi
}

# GNU df rejects -T together with --output, so the filesystem type is requested as an output field.
df_brief() { df -h --output=fstype,size,avail -- "$1" 2>/dev/null | awk 'NR == 2 { print $1 ", size " $2 ", avail " $3 }'; }

collect_disk() {
  local ws="${GITHUB_WORKSPACE:-$PWD}" tmp="${RUNNER_TEMP:-/tmp}"
  if is_linux; then
    kv disk_workspace "$(df_brief "$ws")"
    kv disk_runner_temp "$(df_brief "$tmp")"
    kv disk_tmp "$(df_brief /tmp)"
    [[ -d /var/lib/docker ]] && kv disk_docker_root "$(df_brief /var/lib/docker)"
    echo "--- df -hT"
    df -hT "$ws" "$tmp" /tmp "${HOME:-/}" 2>/dev/null
  else
    echo "--- df -h"
    df -h "$ws" "$tmp" 2>/dev/null
  fi
}

collect_limits() {
  kv ulimit_nofile "$(ulimit -n 2>/dev/null)"
  kv ulimit_nproc "$(ulimit -u 2>/dev/null)"
  [[ "$CG_VER" == 2 ]] && kv pids_limit "$(rd "$CG/pids.max")"
  kv threads_max "$(rd /proc/sys/kernel/threads-max)"
  kv max_map_count "$(rd /proc/sys/vm/max_map_count)"
  kv inotify_max_watches "$(rd /proc/sys/fs/inotify/max_user_watches)"
}

# ver <name> <command...>: logs the first line of a tool's version output, if it is installed.
ver() {
  local name="$1"
  shift
  have "$1" || return 0
  kv "tool_${name//-/_}" "$("$@" 2>&1 | head -n1)"
}

collect_tools() {
  ver java java -version
  ver node node --version
  ver pnpm pnpm --version
  ver go go version
  ver git git --version
  ver python3 python3 --version
  ver jq jq --version
  ver docker docker --version
  ver docker-compose docker compose version
}

collect_docker() {
  have docker || {
    echo "  docker not installed"
    return 0
  }
  kv docker_host "${DOCKER_HOST:-default socket}"
  local -a t=()
  have timeout && t=(timeout 10)
  local out
  if out="$("${t[@]}" docker info --format '{{.ServerVersion}}|{{.Driver}}|{{.CgroupDriver}}|{{.CgroupVersion}}|{{.NCPU}}|{{.MemTotal}}|{{.DockerRootDir}}|{{.OperatingSystem}}' 2>&1)"; then
    local ver drv cgd cgv ncpu mem root dos
    IFS='|' read -r ver drv cgd cgv ncpu mem root dos <<<"$out"
    kv docker_server "$ver"
    kv docker_storage_driver "$drv"
    kv docker_cgroup "$cgd/v$cgv"
    # A Docker-in-Docker daemon can sit in a different container with different limits than the job.
    kv docker_ncpu "$ncpu"
    kv docker_mem_mib "$(mib "$mem")"
    kv docker_root "$root"
    kv docker_os "$dos"
  else
    kv docker_daemon "unreachable: $(echo "$out" | head -n1)"
  fi
}

collect_load() {
  local l1 l5 l15
  read -r l1 l5 l15 _ <<<"$(rd /proc/loadavg)"
  kv node_load_avg "$l1 $l5 $l15"
  kv node_psi_cpu_avg10 "$(psi_avg10 /proc/pressure/cpu some)"
  kv node_psi_mem_avg10 "$(psi_avg10 /proc/pressure/memory some)"
  kv node_psi_io_avg10 "$(psi_avg10 /proc/pressure/io some)"
  [[ -n "$QUOTA" ]] && kv cgroup_psi_cpu_avg10 "$(psi_avg10 "$CG/cpu.pressure" some)"
}

# ---------------------------------------------------------------------------------------------
# optional micro-benchmark: compare raw speed between runners with identical specs
# ---------------------------------------------------------------------------------------------
benchmark() {
  if have python3; then
    # Spins on SHA-256 from <start> (epoch seconds, 0 = now) for <secs> seconds and prints hashes/s.
    # Plain independent processes rather than multiprocessing: the latter re-imports __main__ in
    # its children under the spawn/forkserver start methods (macOS, Python >= 3.14 on Linux),
    # which does not work for a script read from stdin/-c.
    local spin_py='import hashlib, sys, time
start, secs = float(sys.argv[1]), float(sys.argv[2])
start = start or time.time()
data, n = b"x" * 4096, 0
time.sleep(max(0.0, start - time.time()))
end = start + secs
while time.time() < end:
    for _ in range(200):
        hashlib.sha256(data).digest()
    n += 200
print(f"{n / secs:.0f}")'
    local -a t=()
    have timeout && t=(timeout 30)
    local workers single parallel start i
    workers="$(awk -v e="${EFF:-1}" 'BEGIN { w = int(e); if (w < e) w++; if (w < 1) w = 1; print w }')"
    echo "  CPU: 1 worker, then $workers workers (= effective CPUs), 3 s each"
    single="$("${t[@]}" python3 -c "$spin_py" 0 3)"
    start="$(python3 -c 'import time; print(time.time() + 1.5)')" # all workers start together so they really contend
    for ((i = 1; i <= workers; i++)); do
      "${t[@]}" python3 -c "$spin_py" "$start" 3 >"$STATE_DIR/bench.$i" &
    done
    wait
    parallel="$(cat "$STATE_DIR"/bench.* 2>/dev/null | awk '{ s += $1 } END { if (NR > 0) printf "%.0f", s }')"
    rm -f "$STATE_DIR"/bench.*
    kv bench_cpu_workers "$workers"
    kv bench_cpu_single_hashes_per_s "$single"
    kv bench_cpu_parallel_hashes_per_s "$parallel"
    kv bench_cpu_scaling_pct "$(awk -v s="$single" -v p="$parallel" -v w="$workers" 'BEGIN { if (s > 0 && w > 0) printf "%.0f", 100 * p / (s * w) }')"
  else
    echo "  python3 not found, skipping the CPU benchmark"
  fi
  if is_linux; then
    local f="$STATE_DIR/ddtest" line
    line="$(dd if=/dev/zero of="$f" bs=1M count=512 conv=fdatasync 2>&1 | tail -n1)"
    rm -f "$f"
    kv bench_disk_write_512mib "$(echo "$line" | awk -F', ' '{ print $NF }')"
  fi
}

# ---------------------------------------------------------------------------------------------
# counters, and the end phase report
# ---------------------------------------------------------------------------------------------
snapshot() {
  ctr ts "$(date +%s)"
  local k v res
  if [[ "$CG_VER" == 2 ]]; then
    while read -r k v; do
      case "$k" in usage_usec | nr_periods | nr_throttled | throttled_usec) ctr "cpu_$k" "$v" ;; esac
    done < <(rd "$CG/cpu.stat")
    while read -r k v; do ctr "memevt_$k" "$v"; done < <(rd "$CG/memory.events")
    for res in cpu memory io; do
      ctr "psi_${res}_some" "$(psi_total "$CG/$res.pressure" some)"
      ctr "psi_${res}_full" "$(psi_total "$CG/$res.pressure" full)"
    done
  fi
  ctr node_psi_cpu_some "$(psi_total /proc/pressure/cpu some)"
  local u n s i io irq sirq st
  read -r _ u n s i io irq sirq st _ <<<"$(head -n1 /proc/stat 2>/dev/null)"
  if [[ -n "$st" ]]; then
    ctr node_ticks_total $((u + n + s + i + io + irq + sirq + st))
    ctr node_ticks_steal "$st"
  fi
}

report_usage() {
  if [[ ! -s "$START_COUNTERS" ]]; then
    echo "::notice::collect-runner-info: no 'start' baseline in $STATE_DIR, reporting current values only"
  fi
  local wall_s wall_us=""
  wall_s="$(delta ts)"
  [[ -n "$wall_s" ]] && wall_us=$((wall_s * 1000000))
  kv duration_s "$wall_s"

  echo "--- CPU (cgroup of this job's container)"
  QUOTA="$(cpu_quota)"
  kv cpu_quota_cores "$QUOTA"
  kv cpu_avg_cores_used "$(div "$(delta cpu_usage_usec)" "$wall_us")"
  local periods thr thr_pct
  periods="$(delta cpu_nr_periods)"
  thr="$(delta cpu_nr_throttled)"
  thr_pct="$(pct "$thr" "$periods")"
  if [[ -n "$periods" ]]; then
    kv cpu_throttled_periods "$thr of $periods${thr_pct:+ ($thr_pct%)}"
  fi
  kv cpu_throttled_s "$(awk -v u="$(delta cpu_throttled_usec)" 'BEGIN { if (u != "") printf "%.1f", u / 1e6 }')"

  echo "--- Pressure: % of wall time tasks were stalled (some = at least one, full = all)"
  local res kind d
  for res in cpu memory io; do
    for kind in some full; do
      d="$(delta "psi_${res}_${kind}")"
      [[ -n "$d" ]] && kv "psi_${res}_${kind}_pct" "$(pct "$d" "$wall_us")"
    done
  done

  echo "--- Memory"
  local limit
  limit="$(limit_mib "$(rd "$CG/memory.max")")"
  kv mem_limit_mib "$limit"
  kv mem_current_mib "$(mib "$(rd "$CG/memory.current")")"
  kv mem_peak_mib "$(mib "$(rd "$CG/memory.peak")")"
  kv mem_limit_hits "$(delta memevt_max)"
  local oom_kills
  oom_kills="$(delta memevt_oom_kill)"
  kv oom_kills "$oom_kills"
  [[ "$CG_VER" == 2 ]] && kv pids_current "$(rd "$CG/pids.current")"

  echo "--- Node-wide (shared with the other pods on the node)"
  kv node_cpu_steal_pct "$(pct "$(delta node_ticks_steal)" "$(delta node_ticks_total)")"
  kv node_psi_cpu_some_pct "$(pct "$(delta node_psi_cpu_some)" "$wall_us")"
  local l1 l5 l15
  read -r l1 l5 l15 _ <<<"$(rd /proc/loadavg)"
  kv node_load_avg "$l1 $l5 $l15"

  REPORT_OOM_KILLS="$oom_kills"
  REPORT_THROTTLED_PCT="$thr_pct"
}

# ---------------------------------------------------------------------------------------------
# output
# ---------------------------------------------------------------------------------------------
emit_one_line() {
  local line="RUNNER_INFO phase=$PHASE" k v
  while IFS='=' read -r k v; do
    line+=" $k=\"${v//\"/\'}\""
  done <"$CURRENT"
  echo "$line"
}

write_summary() {
  [[ -n "${GITHUB_STEP_SUMMARY:-}" ]] || return 0
  local k v
  {
    echo "### Runner info (${PHASE}): \`${GITHUB_JOB:-job}\` on \`${RUNNER_NAME:-unknown runner}\`"
    echo
    echo "| Key | Value |"
    echo "| --- | --- |"
    while IFS='=' read -r k v; do
      echo "| \`$k\` | ${v//|/\\|} |"
    done <"$CURRENT"
    echo
  } >>"$GITHUB_STEP_SUMMARY"
}

main() {
  detect_cgroup
  if [[ "$PHASE" == "start" ]]; then
    run_group "Job and runner context" collect_context
    run_group "OS, kernel and virtualisation" collect_os
    run_group "CPU" collect_cpu
    run_group "Memory" collect_memory
    run_group "Disk" collect_disk
    run_group "Process and kernel limits" collect_limits
    run_group "Tool versions" collect_tools
    run_group "Docker daemon" collect_docker
    run_group "Node load and pressure at start (node-wide, shared with other pods)" collect_load
    if [[ "${RUNNER_INFO_BENCHMARK:-false}" == "true" ]]; then
      run_group "Micro-benchmark (RUNNER_INFO_BENCHMARK=true)" benchmark
    fi
    # Last, so that the benchmark's own CPU use is not counted in the start->end deltas.
    snapshot
  else
    snapshot
    run_group "Resource usage since the 'start' phase" report_usage
    if [[ "$REPORT_OOM_KILLS" =~ ^[0-9]+$ && "$REPORT_OOM_KILLS" -gt 0 ]]; then
      echo "::warning::collect-runner-info: the kernel OOM-killed $REPORT_OOM_KILLS process(es) in this job's container"
    fi
    if awk -v p="$REPORT_THROTTLED_PCT" 'BEGIN { exit !(p != "" && p >= 10) }'; then
      echo "::notice::collect-runner-info: CPU was throttled in ${REPORT_THROTTLED_PCT}% of the CFS periods (cgroup quota: ${QUOTA} CPUs)"
    fi
  fi
  run_group "One-line form, grep for RUNNER_INFO to compare runs" emit_one_line
  write_summary
}

main
exit 0
