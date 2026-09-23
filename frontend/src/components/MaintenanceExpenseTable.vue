<script setup lang="ts">
import type { MaintenanceExpense } from '../types/domain';

defineProps<{ expenses: MaintenanceExpense[] }>();
</script>

<template>
  <el-table :data="expenses" size="small" empty-text="暂无保养费用记录">
    <el-table-column prop="paidAt" label="完工时间" width="140" />
    <el-table-column prop="machineCode" label="农机" width="120" />
    <el-table-column prop="title" label="项目" min-width="120" show-overflow-tooltip />
    <el-table-column prop="servicePoint" label="维修点" min-width="120" show-overflow-tooltip />
    <el-table-column prop="actualHours" label="工时/h" width="90" />
    <el-table-column label="费用/元" width="110">
      <template #default="{ row }">
        <span class="font-semibold text-emerald-700">¥{{ row.cost }}</span>
      </template>
    </el-table-column>
    <el-table-column label="下次剩余/h" width="110">
      <template #default="{ row }">{{ row.nextRemainHours }}</template>
    </el-table-column>
  </el-table>
</template>
