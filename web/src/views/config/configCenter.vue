<template>
  <div>
    <div class="n-layout-page-header"><n-card :bordered="false" title="配置中心" /></div>
    <n-card :bordered="false" class="mt-4 proCard">
      <BasicTable :columns="columns" :request="loadDataTable" :row-key="(row: ConfigCenterRow) => row.bindingId" ref="actionRef"
        :scroll-x="tableScrollX">
        <template #tableTitle>
          <n-button type="primary" @click="handleCreate" v-permission="{ action: ['config.version.publish'] }">新建绑定</n-button>
        </template>
      </BasicTable>
    </n-card>
    <basicModal @register="modalRegister" @on-ok="handleSubmit"><BasicForm @register="registerForm" class="pt-5" /></basicModal>
  </div>
</template>

<script lang="ts" setup>
  import { computed, h, ref } from 'vue';
  import { NTag, useMessage } from 'naive-ui';
  import { BasicTable, TableAction } from '@/components/Table';
  import { basicModal, useModal } from '@/components/Modal';
  import { BasicForm, useForm } from '@/components/Form';
  import { listConfigCenter, upsertConfigCenter, setConfigCenterEnabled } from '@/api/config';

  const message = useMessage();
  const actionRef = ref();

  interface ConfigCenterRow { bindingId: number; bindingName: string; providerType: string; endpoint: string; namespace: string; authMode: string; lastSyncStatus: string; enabled: boolean; }

  const columns = [
    { title: 'ID', key: 'bindingId', width: 80 },
    { title: '名称', key: 'bindingName', width: 140 },
    { title: '提供者', key: 'providerType', width: 100 },
    { title: '端点', key: 'endpoint', width: 200 },
    { title: '命名空间', key: 'namespace', width: 140 },
    { title: '认证方式', key: 'authMode', width: 100 },
    { title: '同步状态', key: 'lastSyncStatus', width: 100 },
    { title: '启用', key: 'enabled', width: 70, render: (row: ConfigCenterRow) => h(NTag, { type: row.enabled ? 'success' : 'default' }, () => row.enabled ? '是' : '否') },
    { title: '操作', key: 'action', width: 160, render: (row: ConfigCenterRow) => h(TableAction, { actions: [
      { label: '编辑', auth: ['config.version.publish'], onClick: () => handleEdit(row) },
      { label: row.enabled ? '禁用' : '启用', auth: ['config.version.publish'], onClick: () => toggle(row) },
    ]})},
  ];

  const loadDataTable = async (res: Record<string, unknown>) => await listConfigCenter(res as Parameters<typeof listConfigCenter>[0]);
  const tableScrollX = computed(() => columns.reduce((s, c) => s + ((c.width as number) || 150), 0));
  function reloadTable() { actionRef.value.reload(); }

  const schemas = [
    { field: 'bindingName', label: '名称', component: 'NInput', rules: [{ required: true }] },
    { field: 'providerType', label: '提供者', component: 'NInput', defaultValue: 'apollo' },
    { field: 'endpoint', label: '端点', component: 'NInput' },
    { field: 'namespace', label: '命名空间', component: 'NInput', defaultValue: 'application' },
    { field: 'authMode', label: '认证方式', component: 'NInput', defaultValue: 'token' },
    { field: 'token', label: 'Token', component: 'NInput' },
    { field: 'enabled', label: '启用', component: 'NSwitch', defaultValue: true },
    { field: 'priority', label: '优先级', component: 'NInputNumber', defaultValue: 0 },
  ];
  const editingId = ref<number>(0);
  const [registerForm, { submit, setFieldsValue }] = useForm({ schemas, labelWidth: 100, layout: 'horizontal', showActionButtonGroup: false, gridProps: { cols: 1 } });
  const [modalRegister, { openModal, closeModal, setSubLoading }] = useModal({ title: '配置中心', subBtuText: '保存' });

  function handleCreate() { editingId.value = 0; openModal(); }
  function handleEdit(row: ConfigCenterRow) { editingId.value = row.bindingId; setFieldsValue(row); openModal(); }
  async function handleSubmit() {
    try { await submit(); await upsertConfigCenter({ ...formMethods.getFieldsValue(), bindingId: editingId.value || undefined }); message.success('保存成功'); closeModal(); reloadTable(); }
    catch (e: unknown) { if (e instanceof Error) message.error(e.message); setSubLoading(false); }
  }
  async function toggle(row: ConfigCenterRow) { await setConfigCenterEnabled({ bindingId: row.bindingId, enabled: !row.enabled }); message.success('操作成功'); reloadTable(); }
</script>
