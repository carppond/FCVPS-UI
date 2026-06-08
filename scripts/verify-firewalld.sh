#!/usr/bin/env bash
# verify-firewalld.sh — Validate that the REAL firewall-cmd behaves the way the
# hub's firewalldBackend assumes (exit codes + --list-ports output format).
#
# It runs the exact command sequence internal/firewall/backends.go uses, on a
# SAFE throwaway port (9999/tcp), and restores state afterwards. Paste the
# output back so the backend can be confirmed/fixed before release.
#
# Run on a Fedora/Rocky/Alma host, OR in a privileged Fedora container on any
# machine (see the docker one-liner the assistant gave you). Never touches SSH
# or any existing port.
set -u

TEST_PORT=9999
PASS=0
FAIL=0

say()  { printf '\n\033[1m== %s ==\033[0m\n' "$*"; }
ok()   { printf '  \033[32mPASS\033[0m %s\n' "$*"; PASS=$((PASS+1)); }
bad()  { printf '  \033[31mFAIL\033[0m %s\n' "$*"; FAIL=$((FAIL+1)); }
note() { printf '  • %s\n' "$*"; }

# sudo prefix when not root (mirrors the hub's behaviour).
SUDO=""
[ "$(id -u)" -ne 0 ] && SUDO="sudo -n"

run() { # run <label> <cmd...> — prints exit code + raw output
  local label="$1"; shift
  local out rc
  out="$($SUDO "$@" 2>&1)"; rc=$?
  printf '  $ %s\n    exit=%d | out=%q\n' "$*" "$rc" "$out"
  LAST_OUT="$out"; LAST_RC=$rc
}

# ── 0. ensure a running firewalld ───────────────────────────────────────────
# In a container we may bootstrap one (install + iptables backend + dbus +
# daemon). On a REAL host we NEVER touch the config — if the daemon is down we
# just ask the operator to start it, so we don't silently flip their backend.
in_container() { [ -f /.dockerenv ] || grep -qaE 'docker|containerd|kubepods' /proc/1/cgroup 2>/dev/null; }

if command -v firewall-cmd >/dev/null 2>&1 && [ "$($SUDO firewall-cmd --state 2>/dev/null)" = "running" ]; then
  : # already running — normal real-host path
elif in_container; then
  say "容器环境:自举 firewalld(安装 + iptables 后端 + dbus + 守护进程)"
  command -v dnf >/dev/null 2>&1 && \
    $SUDO dnf -y install firewalld dbus-daemon iptables iptables-legacy iproute >/dev/null 2>&1 || true
  [ -s /etc/machine-id ] || $SUDO dbus-uuidgen --ensure=/etc/machine-id 2>/dev/null || true
  # Docker Desktop / LinuxKit kernels lack nftables → use the iptables backend.
  [ -f /etc/firewalld/firewalld.conf ] && \
    $SUDO sed -i 's/^FirewallBackend=.*/FirewallBackend=iptables/' /etc/firewalld/firewalld.conf
  $SUDO update-alternatives --set iptables /usr/sbin/iptables-legacy >/dev/null 2>&1 || true
  $SUDO mkdir -p /run/dbus
  $SUDO dbus-daemon --system --fork >/dev/null 2>&1 || true
  $SUDO /usr/sbin/firewalld --nofork >/tmp/firewalld.log 2>&1 &
  for _ in $(seq 1 15); do
    [ "$($SUDO firewall-cmd --state 2>/dev/null)" = "running" ] && break
    sleep 1
  done
fi

if ! command -v firewall-cmd >/dev/null 2>&1; then
  bad "firewall-cmd 不存在 — 这台机器不是 firewalld 系,换 Fedora/Rocky/Alma"
  exit 1
fi
if [ "$($SUDO firewall-cmd --state 2>/dev/null)" != "running" ]; then
  bad "firewalld 守护进程未运行 — 真机请先 'systemctl start firewalld' 再跑本脚本"
  [ -f /tmp/firewalld.log ] && tail -6 /tmp/firewalld.log
  exit 1
fi

# ── 1. --state (active 检测) ────────────────────────────────────────────────
say "1. firewall-cmd --state(active 检测)"
run state firewall-cmd --state
if echo "$LAST_OUT" | grep -q "running"; then
  ok "--state 含 'running' → backend.active() 会判定 active=true"
else
  bad "--state 未返回 'running'(out=$LAST_OUT) — 守护进程没起来,后续不可信"
fi

# ── 2. --list-ports 初始 ────────────────────────────────────────────────────
say "2. firewall-cmd --list-ports(parseFirewalldPorts 的输入)"
run list_before firewall-cmd --list-ports
note "解析器把它按空格切成 port/proto;当前内容如上(可能为空)"
BEFORE="$LAST_OUT"

# ── 3. 放行:runtime + permanent 双写(backend.allow)────────────────────────
say "3. 放行 ${TEST_PORT}/tcp(--add-port 与 --permanent --add-port)"
run add_runtime   firewall-cmd --add-port=${TEST_PORT}/tcp
A1=$LAST_RC
run add_permanent firewall-cmd --permanent --add-port=${TEST_PORT}/tcp
A2=$LAST_RC
[ "$A1" -eq 0 ] && ok "runtime --add-port exit=0" || bad "runtime --add-port exit=$A1"
[ "$A2" -eq 0 ] && ok "permanent --add-port exit=0" || bad "permanent --add-port exit=$A2"

run list_after_add firewall-cmd --list-ports
if echo "$LAST_OUT" | grep -qw "${TEST_PORT}/tcp"; then
  ok "放行后 --list-ports 含 ${TEST_PORT}/tcp(ListRules 会显示它)"
else
  bad "放行后列表里没有 ${TEST_PORT}/tcp(out=$LAST_OUT)"
fi

# ── 4. 幂等:再放行一次应不报错 ─────────────────────────────────────────────
say "4. 重复放行(幂等性 — backend 不应把 ALREADY_ENABLED 当失败)"
run add_again firewall-cmd --permanent --add-port=${TEST_PORT}/tcp
if [ "$LAST_RC" -eq 0 ]; then
  ok "重复 --permanent --add-port exit=0(幂等 OK)"
else
  note "重复放行 exit=$LAST_RC out=$LAST_OUT — 若非 0,allow() 会报错,需在 backend 容忍 ALREADY_ENABLED"
fi

# ── 5. 删除:runtime + permanent 双删(backend.remove)───────────────────────
say "5. 删除 ${TEST_PORT}/tcp(--remove-port 与 --permanent --remove-port)"
run del_runtime   firewall-cmd --remove-port=${TEST_PORT}/tcp
run del_permanent firewall-cmd --permanent --remove-port=${TEST_PORT}/tcp
D2=$LAST_RC
[ "$D2" -eq 0 ] && ok "permanent --remove-port exit=0" || bad "permanent --remove-port exit=$D2"

run list_after_del firewall-cmd --list-ports
if echo "$LAST_OUT" | grep -qw "${TEST_PORT}/tcp"; then
  bad "删除后 ${TEST_PORT}/tcp 仍在列表(out=$LAST_OUT)"
else
  ok "删除后 ${TEST_PORT}/tcp 已消失"
fi

# ── 6. 状态还原确认 ─────────────────────────────────────────────────────────
say "6. 还原确认(应与初始一致)"
run list_final firewall-cmd --list-ports
if [ "$LAST_OUT" = "$BEFORE" ]; then
  ok "端口列表已还原到初始状态"
else
  note "初始=[$BEFORE] 现在=[$LAST_OUT](若不同,请手动确认 ${TEST_PORT} 已清掉)"
fi

# ── 汇总 ────────────────────────────────────────────────────────────────────
printf '\n\033[1m==== 结果:%d PASS / %d FAIL ====\033[0m\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ] && echo "firewall-cmd 行为与 backend 假设一致 ✅" || echo "有偏差,把上面输出贴给助手 ⚠"
exit "$FAIL"
