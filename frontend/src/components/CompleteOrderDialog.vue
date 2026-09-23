<script setup lang="ts">
import { reactive, ref, watch } from 'vue';
import type { FormInstance, FormRules } from 'element-plus';
import type { MaintenanceOrder } from '../types/domain';

const props = defineProps<{ modelValue: boolean; order: MaintenanceOrder | null }>();
const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void;
  (e: 'submit', payload: { orderId: string; actualHours: number; cost: number; nextRemainHours: number }): void;
}>();

const formRef = ref<FormInstance>();
const submitting = defineModel<boolean>('submitting', { default: false });

const form = reactive({
  actualHours: 0,
  cost: 0,
  nextRemainHours: 100,
});

const rules: FormRules = {
  actualHours: [{ type: 'number', min: 0, message: '工时不能为负', trigger: 'blur' }],
  cost: [{ type: 'number', min: 0, message: '费用不能为负', trigger: 'blur' }],
  nextRemainHours: [{ type: 'number', min: 0.01, message: '下次剩余小时必须大于 0', trigger: 'blur' }],
};

watch(
  () => props.modelValue,
  (visible) => {
    if (visible) {
      form.actualHours = 0;
      form.cost = 0;
      form.nextRemainHours = 100;
      formRef.value?.clearValidate();
    }
  },
);

const handleClose = () => emit('update:modelValue', false);

const handleSubmit = async () => {
  if (!props.order || !formRef.value) return;
  const valid = await formRef.value.validate().catch(() => false);
  if (!valid) return;
  emit('submit', {
    orderId: props.order.id,
    actualHours: Number(form.actualHours),
    cost: Number(form.cost),
    nextRemainHours: Number(form.nextRemainHours),
  });
};
</script>

<template>
  <el-dialog
    :model-value="modelValue"
    title="保养工单完工"
    width="420px"
    :close-on-click-modal="false"
    @update:model-value="emit('update:modelValue', $event)"
    @close="handleClose"
  >
    <div v-if="order" class="mb-4 rounded-md bg-slate-50 p-3 text-sm text-slate-600">
      <p><strong>{{ order.machineCode }} · {{ order.title }}</strong></p>
      <p>计划 {{ order.planDate }} · {{ order.servicePoint }}</p>
    </div>
    <el-form ref="formRef" :model="form" :rules="rules" label-width="112px">
      <el-form-item label="实际工时/h" prop="actualHours">
        <el-input-number v-model="form.actualHours" :min="0" :precision="1" :step="0.5" class="w-full" />
      </el-form-item>
      <el-form-item label="费用/元" prop="cost">
        <el-input-number v-model="form.cost" :min="0" :precision="2" :step="50" class="w-full" />
      </el-form-item>
      <el-form-item label="下次剩余小时" prop="nextRemainHours">
        <el-input-number v-model="form.nextRemainHours" :min="0.01" :precision="1" :step="10" class="w-full" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="handleClose">取消</el-button>
      <el-button type="success" :loading="submitting" @click="handleSubmit">确认完工</el-button>
    </template>
  </el-dialog>
</template>
