package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const swaggerJSON = `{
  "swagger": "2.0",
  "info": {
    "title": "农机调度管理系统 API",
    "description": "农机资源管理、作业任务调度、实时轨迹监控、作业统计与维修保养提醒。",
    "version": "1.0.0"
  },
  "basePath": "/api/v1",
  "schemes": ["http", "ws"],
  "paths": {
    "/auth/login": { "post": { "summary": "登录", "tags": ["auth"] } },
    "/auth/me": { "get": { "summary": "当前用户", "tags": ["auth"] } },
    "/dashboard/overview": { "get": { "summary": "调度看板总览", "tags": ["dashboard"] } },
    "/dashboard/tasks/{id}/dispatch": { "post": { "summary": "一键派单", "tags": ["dashboard"] } },
    "/dashboard/reports/work/export": { "get": { "summary": "作业报表导出", "tags": ["dashboard"] } },
    "/maintenance/orders": { "post": { "summary": "从保养提醒开单（作业中或已有未结束工单则拒绝并留痕）", "tags": ["maintenance"] } },
    "/maintenance/orders/{id}/complete": { "post": { "summary": "完工填报：工时、费用、下次剩余小时，工单/费用/提醒/农机空闲同事务生效，重复完工不产生第二笔费用", "tags": ["maintenance"] } },
    "/maintenance/orders/{id}/cancel": { "post": { "summary": "取消未完工工单：释放农机并恢复提醒", "tags": ["maintenance"] } }
  }
}`

// SwaggerJSON 提供 swagger.json。
func (h *HealthHandler) SwaggerJSON(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	c.String(http.StatusOK, swaggerJSON)
}
