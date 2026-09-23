// 保养工单状态
export const MAINTENANCE_ORDER_STATUS = {
  PROCESSING: '处理中',
  DONE: '已完工',
  CANCELED: '已取消',
  REJECTED: '已拒绝',
} as const;

// 保养提醒节点状态
export const MAINTENANCE_REMINDER_STATUS = {
  OPEN: '待开单',
  PROCESSING: '处理中',
  DONE: '已完工',
} as const;

// 拒绝原因（与后端常量保持一致）
export const REJECT_REASON = {
  MACHINE_WORKING: '农机作业中，暂不能进场保养',
  ORDER_OPEN: '该农机已有未结束工单，请先完工或取消',
  REMINDER_STATE: '提醒当前状态不允许开单',
} as const;

// 保养相关标签颜色
export const MAINTENANCE_STATUS_COLORS: Record<string, string> = {
  待开单: 'danger',
  处理中: 'warning',
  已完工: 'success',
  已取消: 'info',
  已拒绝: 'danger',
};
