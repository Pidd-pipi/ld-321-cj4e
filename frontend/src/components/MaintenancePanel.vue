<script setup lang="ts">
import { ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import CompleteOrderDialog from './CompleteOrderDialog.vue';
import CreateOrderDialog from './CreateOrderDialog.vue';
import MaintenanceExpenseTable from './MaintenanceExpenseTable.vue';
import MaintenanceList from './MaintenanceList.vue';
import MaintenanceOrderTable from './MaintenanceOrderTable.vue';
import {
  cancelMaintenanceOrder,
  completeMaintenanceOrder,
  createMaintenanceOrder,
} from '../services/maintenance.service';
import type {
  MaintenanceExpense,
  MaintenanceOrder,
  MaintenanceReminder,
} from '../types/domain';

const props = defineProps<{
  reminders: MaintenanceReminder[];
  orders: MaintenanceOrder[];
  expenses: MaintenanceExpense[];
}>();
const emit = defineEmits<{ (e: 'changed'): void }>();

const createVisible = ref(false);
const completeVisible = ref(false);
const submitting = ref(false);
const activeReminder = ref<MaintenanceReminder | null>(null);
const activeOrder = ref<MaintenanceOrder | null>(null);

const openCreate = (reminder: MaintenanceReminder) => {
  activeReminder.value = reminder;
  createVisible.value = true;
};

const handleCreate = async (payload: { reminderId: string; planDate: string; servicePoint: string }) => {
  submitting.value = true;
  try {
    const result = await createMaintenanceOrder(payload.reminderId, {
      planDate: payload.planDate,
      servicePoint: payload.servicePoint,
    });
    if (result.order.status === '已拒绝') {
      ElMessage.warning(result.message);
    } else {
      ElMessage.success(result.message);
      createVisible.value = false;
    }
    emit('changed');
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '开单失败');
  } finally {
    submitting.value = false;
  }
};

const openComplete = (order: MaintenanceOrder) => {
  activeOrder.value = order;
  completeVisible.value = true;
};

const handleComplete = async (payload: {
  orderId: string;
  actualHours: number;
  cost: number;
  nextRemainHours: number;
}) => {
  submitting.value = true;
  try {
    const result = await completeMaintenanceOrder(payload.orderId, {
      actualHours: payload.actualHours,
      cost: payload.cost,
      nextRemainHours: payload.nextRemainHours,
    });
    if (result.duplicate) {
      ElMessage.warning(result.message);
    } else {
      ElMessage.success(result.message);
      completeVisible.value = false;
    }
    emit('changed');
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '完工失败');
  } finally {
    submitting.value = false;
  }
};

const handleCancel = async (order: MaintenanceOrder) => {
  try {
    await ElMessageBox.confirm(
      `确认取消 ${order.machineCode} 的保养工单？取消后农机将释放、提醒恢复待开单。`,
      '取消保养工单',
      { type: 'warning', confirmButtonText: '确认取消', cancelButtonText: '再想想' },
    );
  } catch {
    return;
  }
  try {
    const result = await cancelMaintenanceOrder(order.id);
    ElMessage.success(result.message);
    emit('changed');
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '取消失败');
  }
};
</script>

<template>
  <section class="space-y-4">
    <div class="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
      <h2 class="mb-3 text-lg font-black">维修保养提醒</h2>
      <MaintenanceList :reminders="props.reminders" @create="openCreate" />
    </div>

    <div class="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
      <h2 class="mb-3 text-lg font-black">保养工单</h2>
      <MaintenanceOrderTable :orders="props.orders" @complete="openComplete" @cancel="handleCancel" />
    </div>

    <div class="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
      <h2 class="mb-3 text-lg font-black">保养费用记录</h2>
      <MaintenanceExpenseTable :expenses="props.expenses" />
    </div>

    <CreateOrderDialog
      v-model="createVisible"
      v-model:submitting="submitting"
      :reminder="activeReminder"
      @submit="handleCreate"
    />
    <CompleteOrderDialog
      v-model="completeVisible"
      v-model:submitting="submitting"
      :order="activeOrder"
      @submit="handleComplete"
    />
  </section>
</template>
