<script setup lang="ts">
import { ref } from 'vue';
import { ElMessage } from 'element-plus';
import { STATUS_COLORS } from '../constants/app.constants';
import { createMaintenanceOrder } from '../services/maintenance.service';
import type { MaintenanceReminder } from '../types/domain';
import CreateOrderDialog from './CreateOrderDialog.vue';

const props = defineProps<{
  reminders: MaintenanceReminder[];
  onRefresh: () => Promise<void>;
}>();

const dialogVisible = ref(false);
const target = ref<MaintenanceReminder | null>(null);
const submitting = ref(false);

const openDialog = (reminder: MaintenanceReminder) => {
  target.value = reminder;
  dialogVisible.value = true;
};

const handleSubmit = async (planDate: string, repairPoint: string) => {
  if (!target.value) {
    return;
  }
  submitting.value = true;
  try {
    const result = await createMaintenanceOrder({
      reminderId: target.value.id,
      planDate,
      repairPoint,
    });
    if (result.status === '已拒绝') {
      ElMessage.warning(`开单被拒绝：${result.rejectReason}`);
    } else {
      ElMessage.success(result.message);
    }
    dialogVisible.value = false;
    await props.onRefresh();
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '开单失败');
  } finally {
    submitting.value = false;
  }
};
</script>

<template>
  <section class="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
    <h2 class="mb-3 text-lg font-black">维修保养提醒</h2>
    <el-timeline>
      <el-timeline-item
        v-for="item in reminders"
        :key="item.id"
        :type="item.level === 'danger' ? 'danger' : item.level === 'warning' ? 'warning' : 'success'"
        :timestamp="item.dueDate"
      >
        <div class="flex items-start justify-between gap-2">
          <div>
            <div class="flex items-center gap-2">
              <strong>{{ item.machineCode }} · {{ item.title }}</strong>
              <el-tag :type="STATUS_COLORS[item.status] || 'info'" size="small">{{ item.status || '待处理' }}</el-tag>
            </div>
            <p class="text-sm text-slate-600">剩余 {{ item.remainingHours }} 小时 · {{ item.lastServiceRecord }}</p>
          </div>
          <el-button
            v-if="item.status !== '已完工'"
            size="small"
            type="primary"
            :disabled="item.status === '处理中'"
            @click="openDialog(item)"
          >
            {{ item.status === '处理中' ? '已开单' : '开保养工单' }}
          </el-button>
        </div>
      </el-timeline-item>
    </el-timeline>

    <CreateOrderDialog
      v-model="dialogVisible"
      :reminder="target"
      :submitting="submitting"
      @submit="handleSubmit"
    />
  </section>
</template>
