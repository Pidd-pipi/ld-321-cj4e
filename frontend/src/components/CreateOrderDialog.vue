<script setup lang="ts">
import { computed, ref, watch } from 'vue';

const props = defineProps<{
  modelValue: boolean;
  reminder: { machineCode: string; title: string } | null;
  submitting: boolean;
}>();

const emit = defineEmits<{
  'update:modelValue': [value: boolean];
  submit: [planDate: string, repairPoint: string];
}>();

const today = new Date().toISOString().slice(0, 10);

const planDate = ref(today);
const repairPoint = ref('');
const formRef = ref();

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
});

const rules = {
  planDate: [{ required: true, message: '请选择计划日期', trigger: 'change' }],
  repairPoint: [{ required: true, message: '请填写维修点', trigger: 'blur' }],
};

watch(
  () => props.modelValue,
  (open) => {
    if (open) {
      planDate.value = today;
      repairPoint.value = '';
      formRef.value?.clearValidate?.();
    }
  },
);

const confirm = async () => {
  const valid = await formRef.value.validate().catch(() => false);
  if (!valid) {
    return;
  }
  emit('submit', planDate.value, repairPoint.value.trim());
};
</script>

<template>
  <el-dialog v-model="visible" title="开具保养工单" width="420px" :close-on-click-modal="false">
    <el-form
      v-if="reminder"
      ref="formRef"
      label-width="88px"
      :model="{ planDate, repairPoint }"
      :rules="rules"
      @submit.prevent
    >
      <el-form-item label="农机编号">
        <el-input :model-value="reminder.machineCode" disabled />
      </el-form-item>
      <el-form-item label="保养项目">
        <el-input :model-value="reminder.title" disabled />
      </el-form-item>
      <el-form-item label="计划日期" prop="planDate">
        <el-date-picker
          v-model="planDate"
          type="date"
          value-format="YYYY-MM-DD"
          placeholder="选择计划保养日期"
          class="w-full"
        />
      </el-form-item>
      <el-form-item label="维修点" prop="repairPoint">
        <el-input v-model="repairPoint" placeholder="如：北岭农机服务站" clearable />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="confirm">提交开单</el-button>
    </template>
  </el-dialog>
</template>
