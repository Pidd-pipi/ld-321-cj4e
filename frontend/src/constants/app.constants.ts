export const APP_NAME = 'AgriDispatch 农机调度';
export const API_BASE = '/api';
export const STATUS_COLORS: Record<string, string> = {
  空闲: 'success',
  作业中: 'warning',
  维修中: 'danger',
  待派单: 'info',
  已派单: 'warning',
  已完成: 'success',
  待处理: 'warning',
  处理中: 'danger',
  已完工: 'success',
  已取消: 'info',
  已拒绝: 'danger',
};
