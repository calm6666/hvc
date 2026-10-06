<template>
  <basicModal @register="modalRegister" ref="modalRef" @on-ok="okModal">
    <BasicForm @register="registerForm" />
  </basicModal>
</template>

<script lang="ts" setup>
  import { FormSchema, useForm } from '@/components/Form';
  import { basicModal, useModal } from '@/components/Modal';
  import { upsertRole } from '@/api/system/role';
  import { useMessage } from 'naive-ui';

  const message = useMessage();

  /**
   * 新建角色表单 schema。
   * 与后端 upsertRole 请求字段对齐：
   *   { roleKey: string, roleName: string, roleDesc?: string, status: number }
   * 发送时由 HTTP 拦截器的 toSnakeCase 自动转换 key → snake_case。
   */
  const schemas: FormSchema[] = [
    {
      field: 'roleKey',
      component: 'NInput',
      label: '角色标识',
      componentProps: { placeholder: '如 admin、operator' },
      rules: [{ required: true, message: '请输入角色标识', trigger: ['blur'] }],
    },
    {
      field: 'roleName',
      component: 'NInput',
      label: '角色名称',
      componentProps: { placeholder: '如 管理员' },
      rules: [{ required: true, message: '请输入角色名称', trigger: ['blur'] }],
    },
    {
      field: 'roleDesc',
      component: 'NInput',
      label: '角色说明',
      componentProps: { type: 'textarea', placeholder: '选填' },
    },
    {
      field: 'status',
      component: 'NSwitch',
      label: '启用',
      defaultValue: true,
    },
  ];

  const [registerForm, { submit }] = useForm({
    gridProps: { cols: 1 },
    labelWidth: 80,
    layout: 'horizontal',
    showActionButtonGroup: false,
    schemas,
  });

  const [modalRegister, { openModal, closeModal, setSubLoading }] = useModal({
    title: '新增角色',
    subBtuText: '保存',
  });

  const emit = defineEmits(['ok']);

  async function okModal() {
    try {
      const values = await submit();
      if (values) {
        const payload = {
          roleKey: values.roleKey,
          roleName: values.roleName,
          roleDesc: values.roleDesc || '',
          status: values.status ? 1 : 0,
        };
        await upsertRole(payload);
        message.success('角色创建成功');
        closeModal();
        emit('ok');
      }
    } catch (e: unknown) {
      if (e instanceof Error) message.error(e.message);
      setSubLoading(false);
    }
  }

  defineExpose({ openModal });
</script>
