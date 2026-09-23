import { API_BASE } from '../constants/app.constants';
import { AppException } from '../errors/AppException';
import { logger } from '../logger/logger';
import type { MaintenanceOrder } from '../types/domain';

export interface CreateOrderPayload {
  planDate: string;
  servicePoint: string;
}

export interface CompleteOrderPayload {
  actualHours: number;
  cost: number;
  nextRemainHours: number;
}

export interface MaintenanceActionResult {
  order: MaintenanceOrder;
  expense?: unknown;
  duplicate?: boolean;
  message: string;
}

// 统一解包后端 {code, message, data}，业务失败时抛出含后端消息的异常。
const postMaintenance = async (path: string, body?: unknown): Promise<MaintenanceActionResult> => {
  const response = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined,
  });
  const result = await response.json().catch(() => null);
  if (!response.ok || !result || result.code !== 0) {
    const message = result?.message || '保养工单操作失败';
    logger.error('maintenance request failed', path, response.status, message);
    throw new AppException('MAINTENANCE_FAILED', message);
  }
  return result.data as MaintenanceActionResult;
};

// 从保养提醒开单
export const createMaintenanceOrder = (reminderId: string, payload: CreateOrderPayload) =>
  postMaintenance(`/maintenance/reminders/${reminderId}/orders`, payload);

// 完工填写工时、费用、下次剩余小时
export const completeMaintenanceOrder = (orderId: string, payload: CompleteOrderPayload) =>
  postMaintenance(`/maintenance/orders/${orderId}/complete`, payload);

// 取消未完工工单
export const cancelMaintenanceOrder = (orderId: string) =>
  postMaintenance(`/maintenance/orders/${orderId}/cancel`);
