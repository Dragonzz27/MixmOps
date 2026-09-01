<template>
  <div class="page">
    <div class="section-title">
      <div>
        <div class="eyebrow">BACKGROUND OPERATIONS</div>
        <h2>后台任务</h2>
        <span class="muted">自动监听、故障分级、保守修复与人工接力</span>
      </div>
      <n-button type="primary" :loading="loading" @click="load">刷新任务</n-button>
    </div>
    <div class="metric-grid background-metrics">
      <div class="metric"><div><div class="metric-value">{{ tasks.length }}</div><div class="metric-label">全部任务</div></div></div>
      <div class="metric"><div><div class="metric-value">{{ tasks.filter(x => x.severity === 'simple').length }}</div><div class="metric-label">自动修复候选</div></div></div>
      <div class="metric"><div><div class="metric-value">{{ tasks.filter(x => x.incident_id).length }}</div><div class="metric-label">已转故障报告</div></div></div>
      <div class="metric"><div><div class="metric-value">{{ tasks.filter(x => x.status === 'resolved').length }}</div><div class="metric-label">已恢复</div></div></div>
    </div>
    <div class="panel"><n-data-table :columns="columns" :data="tasks" :loading="loading" :pagination="{ pageSize: 10 }" /></div>
  </div>
</template>

<script setup>
import { h, ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useNotification } from 'naive-ui'
import { backgroundApi } from '@/api'

const router = useRouter()
const notification = useNotification()
const tasks = ref([])
const loading = ref(false)
const terminal = new Set(['resolved', 'waiting_human', 'failed', 'cancelled'])
const severityLabel = value => ({ simple: '简单', severe: '严重', unknown: '未知', critical: '严重', warning: '告警' }[value] || value || '未知')
const severityType = value => value === 'simple' ? 'success' : (value === 'severe' || value === 'critical' ? 'error' : value === 'warning' ? 'warning' : 'default')
const statusLabel = value => ({ received: '已接收', collecting_evidence: '采集证据中', analyzing: '分析中', classifying: '严重度判断中', policy_evaluating: '策略评估', auto_repair_pending: '等待自动修复', auto_repairing: '自动修复中', waiting_verification: '验证恢复中', resolved: '已恢复', incident_created: '已生成故障报告', waiting_human: '等待人工介入', failed: '处理失败', cancelled: '已取消' }[value] || value || '未知')
const columns = [
  { title: '告警', key: 'alert_name' },
  { title: '级别', key: 'severity', render: row => h('n-tag', { type: severityType(row.severity), size: 'small' }, { default: () => severityLabel(row.severity) }) },
  { title: '处理轮次', key: 'current_round', render: row => `${row.current_round || 0}/${row.max_rounds || 3}` },
  { title: '处理状态', key: 'status', render: row => h('n-tag', { type: row.status === 'resolved' ? 'success' : row.incident_id ? 'warning' : 'default', size: 'small' }, { default: () => statusLabel(row.status) }) },
  { title: '目标 Pod', key: 'target_pod', render: row => row.target_pod || '—' },
  { title: '处置结果', key: 'repair_result', render: row => row.repair_result || row.severity_reason || '等待分析' },
  { title: '创建时间', key: 'created_at', render: row => row.created_at ? new Date(row.created_at).toLocaleString() : '—' },
  { title: '操作', key: 'action', render: row => h('button', { class: 'link-button', onClick: () => router.push(`/app/background/${row.id}`) }, '查看执行详情') }
]
async function load () {
  loading.value = true
  try { tasks.value = (await backgroundApi.list()).tasks || [] } catch (error) { notification.error({ title: '任务加载失败', content: error.message }) } finally { loading.value = false }
}
let timer
onMounted(() => { load(); timer = setInterval(() => { if (tasks.value.some(task => !terminal.has(task.status))) load() }, 5000) })
onUnmounted(() => clearInterval(timer))
</script>
