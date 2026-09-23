<script setup lang="ts">
import { ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { STATUS_COLORS } from '../constants/app.constants';
import {
  cancelMaintenanceOrder,
  completeMaintenanceOrder,
} from '../services/maintenance.service';
import type { MaintenanceCost, MaintenanceOrder } from '../types/domain';
import CompleteOrderDialog from './CompleteOrderDialog.vue';

const props = defineProps<{
  orders: MaintenanceOrder[];
  costs: MaintenanceCost[];
  onRefresh: () => Promise<void>;
}>();

const completeVisible = ref(false);
const currentOrder = ref<MaintenanceOrder | null>(null);
const submitting = ref(false);

const openComplete = (order: MaintenanceOrder) => {
  currentOrder.value = order;
  completeVisible.value = true;
};

const handleComplete = async (payload: {
  actualHours: number;
  cost: number;
  nextRemainHours: number;
}) => {
  if (!currentOrder.value) {
    return;
  }
  submitting.value = true;
  try {
    const result = await completeMaintenanceOrder(currentOrder.value.id, payload);
    ElMessage.success(result.message);
    completeVisible.value = false;
    await props.onRefresh();
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '完工填报失败');
  } finally {
    submitting.value = false;
  }
};

const handleCancel = async (order: MaintenanceOrder) => {
  try {
    await ElMessageBox.confirm(
      `确定取消 ${order.machineCode} 的「${order.title}」工单吗？农机将被释放。`,
      '取消保养工单',
      { type: 'warning', confirmButtonText: '确定取消', cancelButtonText: '再想想' },
    );
  } catch {
    return;
  }
  try {
    const result = await cancelMaintenanceOrder(order.id);
    ElMessage.success(result.message);
    await props.onRefresh();
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '取消失败');
  }
};

const formatTime = (value: string | null): string => {
  if (!value) {
    return '';
  }
  return new Date(value).toLocaleString('zh-CN', { hour12: false });
};
</script>

<template>
  <section class="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
    <h2 class="mb-3 text-lg font-black">保养工单</h2>

    <el-empty v-if="orders.length === 0" description="暂无保养工单" :image-size="64" />

    <div v-else class="grid gap-3">
      <article
        v-for="order in orders"
        :key="order.id"
        class="rounded-md border border-slate-200 p-3"
        :class="{ 'border-red-200 bg-red-50/40': order.status === '已拒绝' }"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <strong>{{ order.machineCode }} · {{ order.title }}</strong>
              <el-tag :type="STATUS_COLORS[order.status] || 'info'" size="small">{{ order.status }}</el-tag>
            </div>
            <p v-if="order.status === '已拒绝' && order.rejectReason" class="mt-1 text-sm text-red-600">
              拒绝原因：{{ order.rejectReason }}
            </p>
            <p class="mt-1 text-sm text-slate-600">
              计划 {{ order.planDate || '—' }} · 维修点：{{ order.repairPoint || '—' }}
            </p>
            <p v-if="order.status === '已完工'" class="mt-1 text-sm text-emerald-700">
              工时 {{ order.actualHours }} 小时 · 费用 ¥{{ order.cost }} · 下次剩余 {{ order.nextRemainHours }} 小时
              <span v-if="order.completedAt">· {{ formatTime(order.completedAt) }}</span>
            </p>
            <p v-if="order.status === '已取消' && order.cancelledAt" class="mt-1 text-sm text-slate-500">
              取消时间：{{ formatTime(order.cancelledAt) }}
            </p>
          </div>
          <div v-if="order.status === '处理中'" class="flex shrink-0 gap-2">
            <el-button size="small" type="success" @click="openComplete(order)">完工填报</el-button>
            <el-button size="small" @click="handleCancel(order)">取消</el-button>
          </div>
        </div>
      </article>
    </div>

    <div v-if="costs.length > 0" class="mt-4">
      <h3 class="mb-2 text-sm font-bold text-slate-700">保养费用记录</h3>
      <el-table :data="costs" size="small" border>
        <el-table-column prop="machineCode" label="农机" width="120" />
        <el-table-column label="费用" width="110">
          <template #default="{ row }">¥{{ row.amount }}</template>
        </el-table-column>
        <el-table-column label="工时" width="90">
          <template #default="{ row }">{{ row.actualHours }}h</template>
        </el-table-column>
        <el-table-column label="下次剩余(小时)" prop="nextRemainHours" width="130" />
        <el-table-column prop="recordedAt" label="记录日期" />
        <el-table-column prop="orderId" label="工单号" min-width="200" show-overflow-tooltip />
      </el-table>
    </div>

    <CompleteOrderDialog
      v-model="completeVisible"
      :order="currentOrder"
      :submitting="submitting"
      @complete="handleComplete"
    />
  </section>
</template>
