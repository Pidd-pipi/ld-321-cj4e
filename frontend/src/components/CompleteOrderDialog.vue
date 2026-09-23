<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';

const props = defineProps<{
  modelValue: boolean;
  order: { machineCode: string; title: string } | null;
  submitting: boolean;
}>();

const emit = defineEmits<{
  'update:modelValue': [value: boolean];
  complete: [payload: { actualHours: number; cost: number; nextRemainHours: number }];
}>();

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
});

const form = reactive({ actualHours: undefined as number | undefined, cost: undefined as number | undefined, nextRemainHours: 100 });
const formRef = ref();

const validateHours = (_rule: unknown, value: number, callback: (err?: Error) => void) => {
  if (value === undefined || value <= 0) {
    callback(new Error('工时必须大于 0'));
    return;
  }
  callback();
};

const validateCost = (_rule: unknown, value: number, callback: (err?: Error) => void) => {
  if (value === undefined || value < 0) {
    callback(new Error('费用不能为负数'));
    return;
  }
  callback();
};

const validateRemain = (_rule: unknown, value: number, callback: (err?: Error) => void) => {
  if (value === undefined || value < 0) {
    callback(new Error('下次剩余小时不能为负数'));
    return;
  }
  callback();
};

const rules = {
  actualHours: [{ required: true, validator: validateHours, trigger: 'blur' }],
  cost: [{ required: true, validator: validateCost, trigger: 'blur' }],
  nextRemainHours: [{ required: true, validator: validateRemain, trigger: 'blur' }],
};

watch(
  () => props.modelValue,
  (open) => {
    if (open) {
      form.actualHours = undefined;
      form.cost = undefined;
      form.nextRemainHours = 100;
      formRef.value?.clearValidate?.();
    }
  },
);

const confirm = async () => {
  const valid = await formRef.value.validate().catch(() => false);
  if (!valid) {
    return;
  }
  emit('complete', {
    actualHours: Number(form.actualHours),
    cost: Number(form.cost),
    nextRemainHours: Number(form.nextRemainHours),
  });
};
</script>

<template>
  <el-dialog v-model="visible" title="保养工单完工填报" width="420px" :close-on-click-modal="false">
    <el-form
      v-if="order"
      ref="formRef"
      label-width="110px"
      :model="form"
      :rules="rules"
      @submit.prevent
    >
      <el-form-item label="农机编号">
        <el-input :model-value="order.machineCode" disabled />
      </el-form-item>
      <el-form-item label="保养项目">
        <el-input :model-value="order.title" disabled />
      </el-form-item>
      <el-form-item label="实际工时" prop="actualHours">
        <el-input-number v-model="form.actualHours" :min="0.1" :precision="1" :step="0.5" class="w-full">
          <template #suffix>小时</template>
        </el-input-number>
      </el-form-item>
      <el-form-item label="维修费用" prop="cost">
        <el-input-number v-model="form.cost" :min="0" :precision="2" :step="50" class="w-full" />
      </el-form-item>
      <el-form-item label="下次剩余小时" prop="nextRemainHours">
        <el-input-number v-model="form.nextRemainHours" :min="0" :precision="1" :step="10" class="w-full" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="success" :loading="submitting" @click="confirm">确认完工</el-button>
    </template>
  </el-dialog>
</template>
