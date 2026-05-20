#!/usr/bin/env node
/**
 * SYNC 一致性检查器
 *
 * 扫每个独立命令文件与 dev-team.md 嵌入版之间的关键不变量是否同步。
 * 不做全文字符串比对——只检查"如果漂移会引起 bug"的硬性内容：
 *   - 输出文件名（如 06-review、07-unit-test-report）
 *   - 数字阈值（300/50/200 行限制、修复循环 2 轮）
 *   - 风格清单（12 个预设）
 *   - 硬约束（禁止裸 hex 等）
 *   - 关键 git 锚点（scaffold-base、round1/round2）
 *
 * 用法：
 *   node scripts/check-sync.mjs
 *
 * 退出码：
 *   0 - 全部通过
 *   1 - 至少一项漂移
 */

import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(__dirname, '..');

/** 加载文件，缓存内容 */
const cache = new Map();
function load(rel) {
  const abs = resolve(ROOT, rel);
  if (!cache.has(abs)) cache.set(abs, readFileSync(abs, 'utf8'));
  return cache.get(abs);
}

/**
 * 一致性规则。
 * mode:
 *   'all'  - files 中每个文件都必须包含 needle（可以是字符串或字符串数组）
 *   'each' - files 中每个文件都必须包含 needles 数组里的每一个
 */
const RULES = [
  // ───── 输出文件名（最高优先级，错位会撞别人的位） ─────
  { name: '01-requirements 文件名', files: ['.claude/commands/pm.md', '.claude/commands/dev-team.md'], mode: 'all', needle: 'docs/01-requirements.md' },
  { name: '02-ui-design 文件名', files: ['.claude/commands/ui.md', '.claude/commands/dev-team.md'], mode: 'all', needle: 'docs/02-ui-design.md' },
  { name: '03-architecture 文件名', files: ['.claude/commands/architect.md', '.claude/commands/dev-team.md'], mode: 'all', needle: 'docs/03-architecture.md' },
  { name: '04-api-contract 文件名', files: ['.claude/commands/api-contract.md', '.claude/commands/dev-team.md'], mode: 'all', needle: 'docs/04-api-contract.md' },
  { name: '05-tech-lead-plan 文件名', files: ['.claude/commands/tech-lead.md', '.claude/commands/dev-team.md'], mode: 'all', needle: 'docs/05-tech-lead-plan.md' },
  { name: '06-review 文件名', files: ['.claude/commands/review.md', '.claude/commands/dev-team.md'], mode: 'all', needle: 'docs/06-review' },
  { name: '07-unit-test-report 文件名', files: ['.claude/commands/test.md', '.claude/commands/dev-team.md'], mode: 'all', needle: '07-unit-test-report.md' },
  { name: '08-integration-test-report 文件名', files: ['.claude/commands/test.md', '.claude/commands/dev-team.md'], mode: 'all', needle: '08-integration-test-report.md' },
  { name: '09-devops 文件名', files: ['.claude/commands/devops.md', '.claude/commands/dev-team.md'], mode: 'all', needle: 'docs/09-devops.md' },
  { name: '00-coding-standards 文件名', files: ['.claude/commands/coding-standards.md', '.claude/commands/dev-team.md'], mode: 'all', needle: 'docs/00-coding-standards.md' },

  // ───── 编码规范数字阈值（出现在 3 处：standalone + dev-team 嵌入 + dev agent prompt） ─────
  { name: '300 行文件上限', files: ['.claude/commands/coding-standards.md', '.claude/commands/dev-team.md'], mode: 'all', needle: '300 行' },
  { name: '50 行函数上限', files: ['.claude/commands/coding-standards.md', '.claude/commands/dev-team.md'], mode: 'all', needle: '50 行' },
  { name: '200 行组件上限', files: ['.claude/commands/coding-standards.md', '.claude/commands/dev-team.md'], mode: 'all', needle: '200 行' },

  // ───── 12 风格清单 ─────
  {
    name: '12 个视觉风格预设',
    files: ['.claude/commands/ui.md', '.claude/commands/dev-team.md'],
    mode: 'each',
    needles: [
      'Minimalism / Swiss',
      'Glassmorphism',
      'Neumorphism',
      'Material 3',
      'iOS Native',
      'Brutalism',
      'Editorial / Magazine',
      '3D / Hyperrealism',
      'Cyberpunk',
      'Memphis',
      'Pastel',
      'Corporate Classic',
    ],
  },

  // ───── Design Tokens 字段集 ─────
  {
    name: 'Design Tokens 关键字段',
    files: ['.claude/commands/ui.md', '.claude/commands/dev-team.md'],
    mode: 'each',
    needles: ['primary', 'neutral-0', 'neutral-900', 'success', 'warning', 'error', 'info'],
  },

  // ───── Review 视觉规范硬约束 ─────
  {
    name: 'Review 裸 hex grep 模式',
    files: ['.claude/commands/review.md', '.claude/commands/dev-team.md'],
    mode: 'all',
    needle: '#[0-9a-fA-F]{3,8}',
  },
  {
    name: 'Review 裸颜色函数 grep 模式',
    files: ['.claude/commands/review.md', '.claude/commands/dev-team.md'],
    mode: 'all',
    needle: '\\brgb\\(',
  },
  {
    name: 'Review 像素白名单（1px hairline）',
    files: ['.claude/commands/review.md', '.claude/commands/dev-team.md'],
    mode: 'all',
    needle: 'hairline',
  },

  // ───── 流水线锚点（dev-team.md 与 dev-team-continue.md 之间） ─────
  { name: 'scaffold-base git tag', files: ['.claude/commands/dev-team.md', '.claude/commands/dev-team-continue.md'], mode: 'all', needle: 'scaffold-base' },
  { name: '修复循环 round1 备份', files: ['.claude/commands/dev-team.md', '.claude/commands/dev-team-continue.md'], mode: 'all', needle: 'round1' },
  { name: '修复循环 round2 备份', files: ['.claude/commands/dev-team.md', '.claude/commands/dev-team-continue.md'], mode: 'all', needle: 'round2' },

  // ───── 跨平台规则（出现 PowerShell 关键词应同时出现 Bash） ─────
  { name: '跨平台命令双写：Bash', files: ['.claude/commands/dev-team.md'], mode: 'all', needle: 'Bash 工具' },
  { name: '跨平台命令双写：PowerShell', files: ['.claude/commands/dev-team.md'], mode: 'all', needle: 'PowerShell' },

  // ───── Grill 阶段（PM）─────
  {
    name: 'Grill 阶段（PM）锚点',
    files: ['.claude/commands/pm.md', '.claude/commands/dev-team.md'],
    mode: 'each',
    needles: ['Grill 阶段', 'docs/CONTEXT.md', 'docs/adr/', '8 轮', 'Flagged ambiguities'],
  },

  // ───── Diagnose 六步法（Review / 修复）─────
  {
    name: 'Review bug/style 分类',
    files: ['.claude/commands/review.md', '.claude/commands/dev-team.md'],
    mode: 'each',
    needles: ['[bug]', '[style]'],
  },
  {
    name: 'Diagnose 六步法关键词',
    files: ['.claude/commands/review.md', '.claude/commands/dev-team.md'],
    mode: 'each',
    needles: ['复现', '最小化', '假设', '插桩', '回归'],
  },

  // ───── Handoff 章节（writer / 阶段 11）─────
  {
    name: 'Handoff 6 节锚点',
    files: ['.claude/commands/writer.md', '.claude/commands/dev-team.md'],
    mode: 'each',
    needles: [
      '## Handoff: 项目当前状态',
      '## Handoff: 关键决策',
      '## Handoff: 领域术语',
      '## Handoff: 已完成',
      '## Handoff: 未完成 / 已知问题',
      '## Handoff: 下一步建议',
    ],
  },
  {
    name: 'docs/11-handoff.md 输出路径',
    files: ['.claude/commands/writer.md', '.claude/commands/dev-team.md'],
    mode: 'all',
    needle: 'docs/11-handoff.md',
  },
];

/** 验证每个独立命令文件都有 SYNC 注释 */
const SYNC_REQUIRED = [
  '.claude/commands/pm.md',
  '.claude/commands/ui.md',
  '.claude/commands/architect.md',
  '.claude/commands/api-contract.md',
  '.claude/commands/tech-lead.md',
  '.claude/commands/coding-standards.md',
  '.claude/commands/review.md',
  '.claude/commands/test.md',
  '.claude/commands/devops.md',
  '.claude/commands/writer.md',
];

const RESULTS = { pass: 0, fail: 0, failures: [] };

function check(rule) {
  const fails = [];
  for (const f of rule.files) {
    const content = load(f);
    if (rule.mode === 'all') {
      if (!content.includes(rule.needle)) {
        fails.push(`${f}: 缺少 ${JSON.stringify(rule.needle)}`);
      }
    } else if (rule.mode === 'each') {
      for (const n of rule.needles) {
        if (!content.includes(n)) {
          fails.push(`${f}: 缺少 ${JSON.stringify(n)}`);
        }
      }
    }
  }
  if (fails.length === 0) {
    RESULTS.pass++;
    return { ok: true };
  } else {
    RESULTS.fail++;
    RESULTS.failures.push({ rule: rule.name, fails });
    return { ok: false, fails };
  }
}

console.log('SYNC 一致性检查 ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n');

// 1. 检查 SYNC 注释是否就位
console.log('[1/2] SYNC 注释存在性');
for (const f of SYNC_REQUIRED) {
  const content = load(f);
  const hasSync = /<!--\s*SYNC:/.test(content);
  if (hasSync) {
    console.log(`  ✓ ${f}`);
    RESULTS.pass++;
  } else {
    console.log(`  ✗ ${f} — 缺 <!-- SYNC: ... --> 注释`);
    RESULTS.fail++;
    RESULTS.failures.push({ rule: 'SYNC 注释存在', fails: [`${f}: 没有 SYNC 注释`] });
  }
}

// 2. 内容不变量
console.log('\n[2/2] 内容不变量');
for (const rule of RULES) {
  const r = check(rule);
  if (r.ok) {
    console.log(`  ✓ ${rule.name}`);
  } else {
    console.log(`  ✗ ${rule.name}`);
    for (const f of r.fails) console.log(`      ${f}`);
  }
}

// 总结
console.log('\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━');
console.log(`通过 ${RESULTS.pass} 项 / 失败 ${RESULTS.fail} 项`);

if (RESULTS.fail > 0) {
  console.log('\n失败详情：');
  for (const f of RESULTS.failures) {
    console.log(`  • ${f.rule}`);
    for (const reason of f.fails) console.log(`      ${reason}`);
  }
  process.exit(1);
} else {
  console.log('全部通过 ✓');
  process.exit(0);
}
