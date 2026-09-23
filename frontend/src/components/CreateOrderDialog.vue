<script setup lang="ts">
import { reactive, ref, watch } from 'vue';
import type { FormInstance, FormRules } from 'element-plus';
import type { MaintenanceReminder } from '../types/domain';

const props = defineProps<{ modelValue: boolean; reminder: MaintenanceReminder | null }>();
const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void;
  (e: 'submit', payload: { reminderId: string; planDate: string; servicePoint: string }): void;
}>();

const formRef = ref<FormInstance>();
const submitting = defineModel<boolean>('submitting', { default: false });

const form = reactive({
  planDate: '',
  servicePoint: '',
});

const rules: FormRules = {
  planDate: [{ required: true, message: '请选择计划日期', trigger: 'change' }],
  servicePoint: [{ required: true, message: '请填写维修点', trigger: 'blur' }],
};

watch(
  () => props.modelValue,
  (visible) => {
    if (visible) {
      form.planDate = new Date().toISOString().slice(0, 10);
      form.servicePoint = '';
      formRef.value?.clearValidate();
    }
  },
);

const handleClose = () => emit('update:modelValue', false);

const handleSubmit = async () => {
  if (!props.reminder || !formRef.value) return;
  const valid = await formRef.value.validate().catch(() => false);
  if (!valid) return;
  emit('submit', {
    reminderId: props.reminder.id,
    planDate: form.planDate,
    servicePoint: form.servicePoint,
  });
};
</script>

<template>
  <el-dialog
    :model-value="modelValue"
    title="创建保养工单"
    width="420px"
    :close-on-click-modal="false"
    @update:model-value="emit('update:modelValue', $event)"
    @close="handleClose"
  >
    <div v-if="reminder" class="mb-4 rounded-md bg-slate-50 p-3 text-sm text-slate-600">
      <p><strong>{{ reminder.machineCode }} · {{ reminder.title }}</strong></p>
      <p>计划到期 {{ reminder.dueDate }} · 剩余 {{ reminder.remainingHours }} 小时</p>
    </div>
    <el-form ref="formRef" :model="form" :rules="rules" label-width="84px">
      <el-form-item label="计划日期" prop="planDate">
        <el-date-picker
          v-model="form.planDate"
          type="date"
          value-format="YYYY-MM-DD"
          placeholder="选择保养日期"
          class="w-full"
        />
      </el-form-item>
      <el-form-item label="维修点" prop="servicePoint">
        <el-input v-model="form.servicePoint" placeholder="如：县农机服务中心" clearable />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="handleClose">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">提交开单</el-button>
    </template>
  </el-dialog>
</template>
