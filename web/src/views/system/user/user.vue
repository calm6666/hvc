<template>
  <div>
    <div class="n-layout-page-header">
      <n-card :bordered="false" title="用户管理" />
    </div>
    <n-card :bordered="false" class="mt-4 proCard">
      <BasicTable :columns="columns" :request="loadDataTable" :row-key="(row: Record<string, unknown>) => row.adminUserId as number" ref="actionRef" :actionColumn="actionColumn"
        :scroll-x="tableScrollX">
        <template #tableTitle>
          <n-button type="primary" @click="handleCreate" v-permission="{ action: ['system.user.create'] }">新建用户</n-button>
        </template>
      </BasicTable>
    </n-card>
    <!-- 新建/编辑用户 -->
    <basicModal @register="modalRegister" @on-ok="handleSubmit"><BasicForm @register="registerForm" class="pt-5" /></basicModal>
    <!-- 分配角色 -->
    <basicModal @register="roleModalRegister" @on-ok="handleRoleSubmit"><p class="pt-3">为用户分配角色</p><n-select v-model:value="selectedRoleId" :options="roleOptions" placeholder="选择角色" /></basicModal>
  </div>
</template>

<script lang="ts" setup>
  import { computed, h, ref, reactive, nextTick } from 'vue';
  import { NTag, NSelect, useMessage } from 'naive-ui';
  import { BasicTable, TableAction } from '@/components/Table';
  import { basicModal, useModal } from '@/components/Modal';
  import { BasicForm, useForm } from '@/components/Form';
  import { listUsers, upsertUser, setUserStatus, bindUserRole } from '@/api/system/user';
  import { getAllRoles } from '@/api/system/role';
  import type { AdminRoleRow } from '@/types/api';
  import type { AllRolesResponse } from '@/types/api';

  const message = useMessage();
  const actionRef = ref();

  /** 管理员用户行数据。对应后端 adminUserView。 */
  interface UserRow { adminUserId: number; username: string; displayName: string; status: number; lastLoginAt: string; lastLoginIp: string; createdAt: string; }

  const columns = [
    { title: 'ID', key: 'adminUserId', width: 80 },
    { title: '用户名', key: 'username', width: 140 },
    { title: '显示名', key: 'displayName', width: 140 },
    { title: '状态', key: 'status', width: 80, render: (row: UserRow) => h(NTag, { type: row.status === 1 ? 'success' : 'default' }, () => row.status === 1 ? '启用' : '禁用') },
    { title: '最后登录', key: 'lastLoginAt', width: 170 },
    { title: '创建时间', key: 'createdAt', width: 170 },
  ];

  const actionColumn = reactive({
    width: 250, title: '操作', key: 'action', fixed: 'right',
    render(record: Record<string, unknown>) { return h(TableAction, { style: 'button', actions: [
      { label: '编辑', auth: ['system.user.create'], onClick: () => handleEdit(record) },
      { label: record.status === 1 ? '禁用' : '启用', auth: ['system.user.update'], onClick: () => handleToggleStatus(record) },
      { label: '分配角色', auth: ['system.user.role_bind'], onClick: () => handleRoleAssign(record) },
    ]});},
  });

  const loadDataTable = async (res: Record<string, unknown>) => await listUsers(res as Parameters<typeof listUsers>[0]);
  const tableScrollX = computed(() => columns.reduce((s, c) => s + ((c.width as number) || 150), 0));
  function reloadTable() { actionRef.value.reload(); }

  // ===== 新建/编辑 =====
  const editingUserId = ref<number>(0);
  const schemas = [
    { field: 'username', label: '用户名', component: 'NInput', rules: [{ required: true }] },
    { field: 'password', label: '密码', component: 'NInput', componentProps: { type: 'password' } },
    { field: 'displayName', label: '显示名', component: 'NInput' },
    { field: 'status', label: '启用', component: 'NSwitch', defaultValue: true },
  ];
  const [registerForm, { submit, setFieldsValue }] = useForm({ schemas, labelWidth: 80, layout: 'horizontal', showActionButtonGroup: false, gridProps: { cols: 1 } });
  const [modalRegister, { openModal, closeModal, setSubLoading }] = useModal({ title: '用户', subBtuText: '保存' });

  function handleCreate() { editingUserId.value = 0; openModal(); }
  function handleEdit(record: Record<string, unknown>) { editingUserId.value = record.adminUserId as number; openModal(); nextTick(() => setFieldsValue({ username: record.username || '', displayName: record.displayName || '', password: '', status: record.status === 1 })); }
  async function handleSubmit() {
    try { const values = await submit(); if (!values) { setSubLoading(false); return; }
      const payload: Record<string, unknown> = { username: values.username as string, displayName: values.displayName as string, status: (values.status as boolean) ? 1 : 0 };
      if (editingUserId.value) { if (values.password) payload.password = values.password as string; } else { if (!values.password) { message.warning('请输入密码'); setSubLoading(false); return; } payload.password = values.password as string; }
      await upsertUser(payload as Parameters<typeof upsertUser>[0]); message.success('保存成功'); closeModal(); reloadTable();
    } catch (e: unknown) { if (e instanceof Error) message.error(e.message); setSubLoading(false); }
  }

  async function handleToggleStatus(row: Record<string, unknown>) { const s = row.status as number; await setUserStatus({ adminUserId: row.adminUserId as number, status: s === 1 ? 0 : 1 }); message.success('操作成功'); reloadTable(); }

  // ===== 分配角色 =====
  const selectedRoleId = ref<number | null>(null);
  const roleOptions = ref<{ label: string; value: number }[]>([]);
  const currentUserId = ref<number>(0);
  const [roleModalRegister, { openModal: openRoleModal, closeModal: closeRoleModal, setSubLoading: setRoleSubLoading }] = useModal({ title: '分配角色', subBtuText: '确定' });

  async function handleRoleAssign(record: Record<string, unknown>) {
    currentUserId.value = record.adminUserId as number;
    selectedRoleId.value = null;
    try { const data = await getAllRoles() as AllRolesResponse; roleOptions.value = (data?.items || []).map((r: AdminRoleRow) => ({ label: `${r.roleName} (${r.roleKey})`, value: r.roleId })); } catch (e) { /* ignore */ }
    openRoleModal();
  }
  async function handleRoleSubmit() {
    if (!selectedRoleId.value) { message.warning('请选择角色'); setRoleSubLoading(false); return; }
    try { await bindUserRole({ adminUserId: currentUserId.value, roleId: selectedRoleId.value }); message.success('角色分配成功'); closeRoleModal(); } catch (e: unknown) { if (e instanceof Error) message.error(e.message); setRoleSubLoading(false); }
  }
</script>
