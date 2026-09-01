<template>
  <div class="page" v-if="task">
    <div class="section-title">
      <div>
        <button class="back-link" @click="router.push('/app/background')">← 返回后台任务</button>
        <div class="eyebrow">PLAN · EXECUTE · OBSERVE · REPLAN</div>
        <h2>{{ task.alert_name || '后台处置任务' }}</h2>
        <span class="muted">{{ task.namespace }} · {{ task.alert_fingerprint }}</span>
      </div>
      <n-space>
        <n-button v-if="task.status === 'failed'" :loading="actionLoading" @click="retry">重试任务</n-button>
        <n-button v-if="!task.incident_id && ['failed', 'waiting_human', 'incident_created'].includes(task.status)" :loading="actionLoading" @click="createIncident">创建故障报告
        </n-button>
        <n-button v-if="!['resolved', 'cancelled'].includes(task.status)" type="error" secondary :loading="actionLoading" @click="cancel">取消任务
        </n-button>
        <n-button v-if="task.incident_id" type="primary" @click="router.push(`/app/incidents/${task.incident_id}`)">查看故障报告</n-button>
      </n-space>
    </div>

    <div class="background-detail-grid">
      <div>
        <section class="panel workflow-overview">
          <div class="section-title">
            <h3>处置进度</h3>
            <n-tag :type="statusType(task.status)">{{ statusLabel(task.status) }}</n-tag>
          </div>
          <div class="round-progress">
            <strong>第 {{ task.current_round || 0 }} / {{ task.max_rounds || 3 }} 轮</strong>
            <span>每轮最多执行一次受控修复，验证失败后重新规划</span>
          </div>
          <div class="workflow-steps">
            <div v-for="step in steps" :key="step.key" :class="['workflow-step', stepState(step.key)]">
              <span class="step-index">{{ step.index }}</span>
              <div><strong>{{ step.label }}</strong><small>{{ step.description }}</small></div>
            </div>
          </div>
        </section>

        <section class="panel">
          <div class="section-title">
            <div><h3>每轮决策快照</h3><span class="muted">Remediation Agent 的 Plan、Policy 和 Observe 结果</span></div>
            <n-button quaternary size="small" :loading="loading" @click="load">刷新</n-button>
          </div>
          <div class="snapshot-grid">
            <article class="snapshot-card">
              <div class="snapshot-title">Plan / Decision</div>
              <div class="snapshot-meta">严重度：<n-tag :type="severityType(task.severity)" size="small">{{ severityLabel(task.severity) }}</n-tag></div>
              <pre>{{ pretty(task.decision) }}</pre>
            </article>
            <article class="snapshot-card">
              <div class="snapshot-title">Policy / Action Plan</div>
              <div class="snapshot-meta">动作：{{ plan.action || 'create_incident' }}</div>
              <pre>{{ pretty(task.plan) }}</pre>
            </article>
            <article class="snapshot-card">
              <div class="snapshot-title">Observe / Verification</div>
              <div class="snapshot-meta">验证结果</div>
              <pre>{{ pretty(task.observation) }}</pre>
            </article>
          </div>
        </section>

        <section class="panel">
          <div class="section-title"><div><h3>执行时间线</h3><span class="muted">所有状态变化、工具证据和动作结果均可审计</span></div></div>
          <n-timeline v-if="timeline.length">
            <n-timeline-item v-for="(event, index) in timeline" :key="`${event.created_at}-${index}`" :type="timelineType(event)" :title="timelineLabel(event)" :time="formatTime(event.created_at)">
              <div class="timeline-status">{{ event.status === 'success' ? '成功' : event.status === 'failed' ? '失败' : event.status }}</div>
              <pre v-if="event.output || event.error" class="timeline-output">{{ event.error || event.output }}</pre>
            </n-timeline-item>
          </n-timeline>
          <n-empty v-else description="暂无时间线记录" />
        </section>
      </div>

      <div>
        <section class="panel evidence-panel">
          <div class="section-title"><h3>当前任务摘要</h3></div>
          <n-descriptions :column="1" size="small">
            <n-descriptions-item label="严重度"><n-tag :type="severityType(task.severity)" size="small">{{ severityLabel(task.severity) }}</n-tag></n-descriptions-item>
            <n-descriptions-item label="告警级别">{{ task.alert_severity || 'unknown' }}
            </n-descriptions-item>
            <n-descriptions-item label="目标 Pod">{{ task.target_pod || '尚未定位' }}
            </n-descriptions-item>
            <n-descriptions-item label="修复状态">{{ task.repair_status || '未执行' }}
            </n-descriptions-item>
            <n-descriptions-item label="修复结果">{{ task.repair_result || '—' }}
            </n-descriptions-item>
            <n-descriptions-item label="故障报告">{{ task.incident_id || '未创建' }}
            </n-descriptions-item>
          </n-descriptions>
        </section>
        <section class="panel"><div class="section-title"><h3>证据快照</h3></div><pre class="log-window evidence-window">{{ pretty(task.evidence) }}</pre></section>
        <section class="panel handoff-card" v-if="task.incident_id">
          <div class="section-title"><h3>已接力 Incident</h3><n-tag type="warning">等待人工
          </n-tag></div>
          <p>后台自动处置未能安全恢复，完整证据已交给故障排查 Agent。</p>
          <n-button type="primary" block @click="router.push(`/app/incidents/${task.incident_id}`)">进入故障排查
          </n-button>
        </section>
      </div>
    </div>
  </div>
  <div class="page" v-else-if="loading"><n-spin size="large" /></div>
  <div class="page" v-else>
    <n-result status="404" title="任务不存在" description="请返回后台任务列表重新选择" />
    <n-button @click="router.push('/app/background')">返回任务列表
    </n-button>
  </div>
</template>

<script setup>
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useNotification } from 'naive-ui'
import { backgroundApi } from '@/api'

const route = useRoute()
const router = useRouter()
const notification = useNotification()
const task = ref(null)
const timeline = ref([])
const loading = ref(false)
const actionLoading = ref(false)
const terminal = new Set(['resolved', 'waiting_human', 'failed', 'cancelled'])

const steps = [
  { index: 1, key: 'plan', label: 'Plan', description: '采集证据并生成结构化处置计划' },
  { index: 2, key: 'policy', label: 'Policy', description: '校验白名单、严重度和目标安全性' },
  { index: 3, key: 'execute', label: 'Execute', description: '执行唯一受控自动动作' },
  { index: 4, key: 'observe', label: 'Observe', description: '重新查询 Pod、Deployment 和告警' },
  { index: 5, key: 'replan', label: 'Replan', description: '恢复失败时进入下一轮或接力 Incident' }
]
const plan = computed(() => parseSnapshot(task.value?.plan))

const severityLabel = value => ({ simple: '简单', severe: '严重', unknown: '未知', critical: '严重', warning: '告警' }[value] || value || '未知')
const severityType = value => value === 'simple' ? 'success' : (value === 'severe' || value === 'critical' ? 'error' : value === 'warning' ? 'warning' : 'default')
const statusLabel = value => ({ received: '已接收', collecting_evidence: '采集证据中', analyzing: '分析中', classifying: '严重度判断中', policy_evaluating: '策略评估', auto_repair_pending: '等待自动修复', auto_repairing: '自动修复中', waiting_verification: '验证恢复中', resolved: '已恢复', incident_created: '已生成故障报告', waiting_human: '等待人工介入', failed: '处理失败', cancelled: '已取消' }[value] || value || '未知')
const statusType = value => value === 'resolved' ? 'success' : ['failed', 'cancelled'].includes(value) ? 'error' : ['incident_created', 'waiting_human'].includes(value) ? 'warning' : 'info'

function parseSnapshot (value) {
  if (!value) return {}
  if (typeof value === 'object') return value
  try { return JSON.parse(value) } catch { return { raw: value } }
}
function pretty (value) { return JSON.stringify(parseSnapshot(value), null, 2) }
function stepState (key) {
  if (!task.value) return ''
  if (key === 'plan') return task.value.decision ? 'completed' : task.value.status === 'received' ? 'active' : ''
  if (key === 'policy') return task.value.plan ? 'completed' : task.value.status === 'policy_evaluating' ? 'active' : ''
  if (key === 'execute') return task.value.repair_status === 'completed' || task.value.repair_status === 'verified' ? 'completed' : ['auto_repair_pending', 'auto_repairing'].includes(task.value.status) ? 'active' : ''
  if (key === 'observe') return task.value.observation ? (task.value.status === 'waiting_verification' ? 'active' : 'completed') : ''
  return ['resolved', 'incident_created', 'waiting_human', 'failed', 'cancelled'].includes(task.value.status) ? 'completed' : task.value.current_round > 1 ? 'active' : ''
}
function timelineLabel (event) { return ({ alert_received: '告警接收', evidence_collected: '证据采集', plan_created: '计划/决策生成', policy_evaluated: 'Policy 策略评估', repair_started: '自动修复开始', repair_completed: '自动修复完成', repair_failed: '自动修复失败', observation_collected: '恢复观测', replan_started: '开始重新规划', incident_created: '接力 Incident', workflow_completed: '工作流完成', remediation_failed: 'Remediation 失败' }[event.event_name] || event.event_name || 'Agent 事件') }
function timelineType (event) { return event.status === 'failed' || event.event_name?.includes('failed') ? 'error' : event.event_name === 'workflow_completed' ? 'success' : 'info' }
function formatTime (value) { return value ? new Date(value).toLocaleString() : '' }

async function load () {
  loading.value = true
  try {
    task.value = await backgroundApi.get(route.params.id)
    const response = await backgroundApi.timeline(route.params.id)
    timeline.value = response.timeline || []
  } catch (error) {
    task.value = null
    notification.error({ title: '任务详情加载失败', content: error.message })
  } finally { loading.value = false }
}
async function retry () { await runAction(() => backgroundApi.retry(route.params.id), '任务已重新调度') }
async function cancel () { await runAction(() => backgroundApi.cancel(route.params.id), '任务已取消') }
async function createIncident () { await runAction(() => backgroundApi.createIncident(route.params.id), '故障报告已创建') }
async function runAction (action, message) {
  actionLoading.value = true
  try { await action(); notification.success({ title: '操作成功', content: message }); await load() } catch (error) { notification.error({ title: '操作失败', content: error.message }) } finally { actionLoading.value = false }
}

let timer
onMounted(() => { load(); timer = setInterval(() => { if (task.value && !terminal.has(task.value.status)) load() }, 5000) })
onUnmounted(() => clearInterval(timer))
</script>
