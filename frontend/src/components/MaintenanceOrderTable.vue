<script setup lang="ts">
import { MAINTENANCE_ORDER_STATUS, MAINTENANCE_STATUS_COLORS } from '../constants/maintenance.constants';
import type { MaintenanceOrder } from '../types/domain';

defineProps<{ orders: MaintenanceOrder[] }>();
const emit = defineEmits<{
  (e: 'complete', order: MaintenanceOrder): void;
  (e: 'cancel', order: MaintenanceOrder): void;
}>();
</script>

<template>
  <el-table :data="orders" size="small" empty-text="暂无保养工单">
    <el-table-column prop="machineCode" label="农机" width="120" />
    <el-table-column prop="title" label="保养项目" min-width="120" show-overflow-tooltip />
    <el-table-column prop="planDate" label="计划日期" width="100" />
    <el-table-column prop="servicePoint" label="维修点" min-width="120" show-overflow-tooltip />
    <el-table-column label="状态" width="90">
      <template #default="{ row }">
        <el-tag :type="MAINTENANCE_STATUS_COLORS[row.status] || 'info'" size="small">{{ row.status }}</el-tag>
      </template>
    </el-table-column>
    <el-table-column label="工时/费用" width="120">
      <template #default="{ row }">
        <span v-if="row.status === MAINTENANCE_ORDER_STATUS.DONE">{{ row.actualHours }}h / ¥{{ row.cost }}</span>
        <span v-else class="text-slate-400">-</span>
      </template>
    </el-table-column>
    <el-table-column label="拒绝原因" min-width="160">
      <template #default="{ row }">
        <span v-if="row.status === MAINTENANCE_ORDER_STATUS.REJECTED" class="text-red-600">{{ row.rejectReason }}</span>
        <span v-else class="text-slate-400">-</span>
      </template>
    </el-table-column>
    <el-table-column label="操作" width="150" fixed="right">
      <template #default="{ row }">
        <template v-if="row.status === MAINTENANCE_ORDER_STATUS.PROCESSING">
          <el-button size="small" type="success" link @click="emit('complete', row)">完工</el-button>
          <el-button size="small" type="warning" link @click="emit('cancel', row)">取消</el-button>
        </template>
        <span v-else class="text-slate-400">-</span>
      </template>
    </el-table-column>
  </el-table>
</template>
