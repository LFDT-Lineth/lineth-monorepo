#!/usr/bin/env bash
# TEMPORARY CI instrumentation (remove before merge).
# Samples the container's cgroup (v2) CPU/memory so the numbers reflect the pod's
# quota/limit, not the host: inside a container /proc/stat and /proc/meminfo show
# the HOST, while /sys/fs/cgroup/* show this pod. Answers "is there CPU/mem headroom
# for more test forks, and where does throttling happen (startup spikes vs steady)?"
#
# Usage:
#   ci-cgroup-stats.sh limits
#   ci-cgroup-stats.sh sample  <csv>   # loops until killed, appends CSV
#   ci-cgroup-stats.sh summary <csv>
set -u
CG=/sys/fs/cgroup

field() { awk -v k="$1" '$1==k{print $2}' "$CG/cpu.stat" 2>/dev/null; }
quota_cores() { awk '{ if ($1=="max") print "0"; else printf "%.3f", $1/$2 }' "$CG/cpu.max" 2>/dev/null; }

case "${1:-}" in
  limits)
    echo "cpu.max    = $(cat "$CG/cpu.max" 2>/dev/null)   (quota/period; quota_cores=$(quota_cores))"
    echo "memory.max = $(cat "$CG/memory.max" 2>/dev/null)"
    echo "host-visible nproc = $(nproc)   (note: host count, not the pod quota)"
    ;;
  sample)
    out="${2:?csv path required}"
    echo "ts,cores_used,quota_cores,cpu_util_pct,throttled_pct,mem_used_mb,jvm_workers" > "$out"
    q=$(quota_cores)
    pu=$(field usage_usec); pt=$(field throttled_usec); pw=$(date +%s%N)
    while true; do
      sleep 5
      nw=$(date +%s%N); u=$(field usage_usec); t=$(field throttled_usec)
      memc=$(cat "$CG/memory.current" 2>/dev/null || echo 0)
      dw=$(( (nw - pw) / 1000 )); [ "$dw" -le 0 ] && dw=1
      du=$(( ${u:-0} - ${pu:-0} )); dt=$(( ${t:-0} - ${pt:-0} ))
      cores=$(awk -v a="$du" -v b="$dw" 'BEGIN{printf "%.2f", a/b}')
      util=$(awk -v a="$du" -v b="$dw" -v q="$q" 'BEGIN{ if(q>0) printf "%.0f", 100*a/b/q }')
      thr=$(awk -v a="$dt" -v b="$dw" 'BEGIN{printf "%.0f", 100*a/b}')
      jw=$(pgrep -fc GradleWorkerMain 2>/dev/null || echo 0)
      echo "$(date +%s),$cores,$q,$util,$thr,$(( ${memc:-0} / 1048576 )),$jw" >> "$out"
      pu=$u; pt=$t; pw=$nw
    done
    ;;
  summary)
    out="${2:?csv path required}"
    awk -F, '
      NR==2{ q=$3 }
      NR>1 { n++; cs+=$2; if($2>pc)pc=$2;
             if($4!=""){ us+=$4; if($4>pu)pu=$4 }
             if($5>0)thrN++; ts+=$5; if($5>pt)pt=$5;
             ms+=$6; if($6>pm)pm=$6; if($7>pj)pj=$7 }
      END  { if(!n){ print "no samples captured"; exit }
             printf "samples=%d (~%.0f min @5s)\n", n, n*5/60;
             printf "quota_cores      = %s\n", q;
             printf "cpu cores used   : avg=%.2f  peak=%.2f\n", cs/n, pc;
             printf "cpu utilization  : avg=%.0f%%  peak=%.0f%%  (of quota -> idle headroom = 100 - avg)\n", us/n, pu;
             printf "cfs throttling   : %d/%d samples throttled, avg=%.0f%% peak=%.0f%% of interval\n", thrN, n, ts/n, pt;
             printf "memory used      : avg=%d MB  peak=%d MB\n", ms/n, pm;
             printf "jvm workers      : peak=%d\n", pj }' "$out"
    ;;
  *) echo "usage: $0 limits|sample <csv>|summary <csv>" >&2; exit 1 ;;
esac
