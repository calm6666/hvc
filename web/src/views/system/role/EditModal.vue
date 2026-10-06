<template>
  <basicModal @register="modalRegister" ref="modalRef" @on-ok="okModal">
    <BasicForm @register="registerForm" />
  </basicModal>
</template>

<script lang="ts" setup>
  import { nextTick } from 'vue';
  import { FormSchema, useForm } from '@/components/Form';
  import { basicModal, useModal } from '@/components/Modal';
  import { upsertRole } from '@/api/system/role';
  import { useMessage } from 'naive-ui';

  const message = useMessage();

  /**
   * 编辑角色表单 schema。
   * 字段与 CreateModal 一致，通过 setFieldsValue 回填现有值。
   * roleKey 在编辑模式下设为只读（不可修改角色标识）。
   */
  const schemas: FormSchema[] = [
    {
      field: 'roleKey',
      component: 'NInput',
      label: '角色标识',
      rules: [{ required: true, message: '请输入角色标识', trigger: ['blur'] }],
      componentProps: { disabled: true },
    },
    {
      field: 'roleName',
      component: 'NInput',
      label: '角色名称',
      rules: [{ required: true, message: '请输入角色名称', trigger: ['blur'] }],
    },
    {
      field: 'roleDesc',
      component: 'NInput',
      label: '角色说明',
      componentProps: { type: 'textarea' },
    },
    {
      field: 'status',
      component: 'NSwitch',
      label: '启用',
      defaultValue: true,
    },
  ];

  const [registerForm, { submit, setFieldsValue }] = useForm({
    gridProps: { cols: 1 },
    labelWidth: 80,
    layout: 'horizontal',
    showActionButtonGroup: false,
    schemas,
  });

  const [modalRegister, { openModal, closeModal, setSubLoading }] = useModal({
    title: '编辑角色',
    subBtuText: '保存',
  });

  const emit = defineEmits(['ok']);

  /** 编辑角色时回填表单，字段名与后端 adminRoleView 对齐 */
  function showModal(record: Recordable) {
    openModal();
    nextTick(() => {
      if (record) {
        setFieldsValue({
          roleKey: record.roleKey || '',
          roleName: record.roleName || '',
          roleDesc: record.roleDesc || '',
          status: record.status === 1,
        });
      }
    });
  }

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
        message.success('角色更新成功');
        closeModal();
        emit('ok');
      }
    } catch (e: unknown) {
      if (e?.message) message.error(e.message);
      setSubLoading(false);
    }
  }

  defineExpose({ showModal });
</script>
