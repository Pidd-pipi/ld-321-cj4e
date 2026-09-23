package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const swaggerJSON = `{
  "swagger": "2.0",
  "info": {
    "title": "农机调度管理系统 API",
    "description": "农机资源管理、作业任务调度、实时轨迹监控、作业统计与维修保养工单。",
    "version": "1.0.0"
  },
  "basePath": "/api/v1",
  "schemes": ["http", "ws"],
  "paths": {
    "/auth/login": { "post": { "summary": "登录", "tags": ["auth"] } },
    "/auth/me": { "get": { "summary": "当前用户", "tags": ["auth"] } },
    "/dashboard/overview": { "get": { "summary": "调度看板总览（含保养工单/费用/提醒节点）", "tags": ["dashboard"] } },
    "/tasks/{id}/dispatch": { "post": { "summary": "一键派单", "tags": ["dashboard"] } },
    "/dashboard/reports/work/export": { "get": { "summary": "作业报表导出", "tags": ["dashboard"] } },
    "/maintenance/reminders/{id}/orders": {
      "post": {
        "summary": "从保养提醒开单（计划日期、维修点；作业中或已有未结束工单时拒绝）",
        "tags": ["maintenance"],
        "parameters": [
          { "name": "id", "in": "path", "required": true, "type": "string" },
          { "name": "body", "in": "body", "required": true, "schema": { "$ref": "#/definitions/CreateMaintenanceOrderRequest" } }
        ]
      }
    },
    "/maintenance/orders/{id}/complete": {
      "post": {
        "summary": "工单完工（工时、费用、下次剩余小时；重复完工不产生第二笔费用）",
        "tags": ["maintenance"],
        "parameters": [
          { "name": "id", "in": "path", "required": true, "type": "string" },
          { "name": "body", "in": "body", "required": true, "schema": { "$ref": "#/definitions/CompleteMaintenanceOrderRequest" } }
        ]
      }
    },
    "/maintenance/orders/{id}/cancel": {
      "post": { "summary": "取消未完工工单（释放农机、恢复提醒）", "tags": ["maintenance"] }
    }
  },
  "definitions": {
    "CreateMaintenanceOrderRequest": {
      "type": "object",
      "required": ["planDate", "servicePoint"],
      "properties": {
        "planDate": { "type": "string", "example": "2026-09-25" },
        "servicePoint": { "type": "string", "example": "县农机服务中心" }
      }
    },
    "CompleteMaintenanceOrderRequest": {
      "type": "object",
      "required": ["nextRemainHours"],
      "properties": {
        "actualHours": { "type": "number", "example": 4.5 },
        "cost": { "type": "number", "example": 1280 },
        "nextRemainHours": { "type": "number", "example": 100 }
      }
    }
  }
}`

// SwaggerJSON 提供 swagger.json。
func (h *HealthHandler) SwaggerJSON(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	c.String(http.StatusOK, swaggerJSON)
}
