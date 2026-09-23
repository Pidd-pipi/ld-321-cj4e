<script setup lang="ts">
import {
  MAINTENANCE_REMINDER_STATUS,
  MAINTENANCE_STATUS_COLORS,
} from '../constants/maintenance.constants';
import type { MaintenanceReminder } from '../types/domain';

defineProps<{ reminders: MaintenanceReminder[] }>();
const emit = defineEmits<{ (e: 'create', reminder: MaintenanceReminder): void }>();

const timelineType = (level: string, status: string) => {
  if (status === MAINTENANCE_REMINDER_STATUS.DONE) return 'success';
  if (status === MAINTENANCE_REMINDER_STATUS.PROCESSING) return 'warning';
  return level === 'danger' ? 'danger' : level === 'warning' ? 'warning' : 'primary';
};
</script>

<template>
  <el-timeline>
    <el-timeline-item
      v-for="item in reminders"
      :key="item.id"
      :type="timelineType(item.level, item.status)"
      :timestamp="item.dueDate"
    >
      <div class="flex items-start justify-between gap-3">
        <div>
          <div class="flex items-center gap-2">
            <strong>{{ item.machineCode }} · {{ item.title }}</strong>
            <el-tag :type="MAINTENANCE_STATUS_COLORS[item.status] || 'info'" size="small">
              {{ item.status }}
            </el-tag>
          </div>
          <p class="mt-1 text-sm text-slate-600">
            剩余 {{ item.remainingHours }} 小时 · {{ item.lastServiceRecord }}
          </p>
          <p v-if="item.status === '处理中' && item.activeOrderId" class="mt-1 text-xs text-amber-700">
            关联工单 {{ item.activeOrderId }}，农机维修中
          </p>
        </div>
        <el-button
          v-if="item.status === MAINTENANCE_REMINDER_STATUS.OPEN"
          size="small"
          type="primary"
          @click="emit('create', item)"
        >
          开单
        </el-button>
      </div>
    </el-timeline-item>
  </el-timeline>
</template>
