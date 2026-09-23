import { API_BASE } from '../constants/app.constants';
import { AppException } from '../errors/AppException';
import type { MaintenanceCost, MaintenanceOrder } from '../types/domain';

export interface CreateOrderPayload {
  reminderId: string;
  planDate: string;
  repairPoint: string;
}

export interface CompleteOrderPayload {
  actualHours: number;
  cost: number;
  nextRemainHours: number;
}

export interface MaintenanceActionResult {
  orderId: string;
  status: string;
  message: string;
  rejectReason?: string;
  costId?: string;
  cost?: number;
  nextRemainHours?: number;
}

const unwrap = async <T>(response: Response, fallbackMessage: string): Promise<T> => {
  const body = await response.json().catch(() => null);
  if (!response.ok || !body) {
    const message = body && typeof body === 'object' && body.message ? body.message : fallbackMessage;
    throw new AppException('MAINTENANCE_FAILED', message);
  }
  if (body && typeof body === 'object' && body.code === 0 && body.data !== undefined) {
    return body.data as T;
  }
  return body as T;
};

export const createMaintenanceOrder = async (payload: CreateOrderPayload): Promise<MaintenanceActionResult> => {
  const response = await fetch(`${API_BASE}/maintenance/orders`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return unwrap<MaintenanceActionResult>(response, '开单失败');
};

export const completeMaintenanceOrder = async (
  orderId: string,
  payload: CompleteOrderPayload,
): Promise<MaintenanceActionResult> => {
  const response = await fetch(`${API_BASE}/maintenance/orders/${orderId}/complete`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return unwrap<MaintenanceActionResult>(response, '完工填报失败');
};

export const cancelMaintenanceOrder = async (orderId: string): Promise<MaintenanceActionResult> => {
  const response = await fetch(`${API_BASE}/maintenance/orders/${orderId}/cancel`, {
    method: 'POST',
  });
  return unwrap<MaintenanceActionResult>(response, '取消工单失败');
};

export type { MaintenanceCost, MaintenanceOrder };
