<template>
  <div>
    <div class="n-layout-page-header"><n-card :bordered="false" title="回调配置" /></div>
    <n-card :bordered="false" class="mt-4 proCard">
      <BasicTable :columns="columns" :request="loadDataTable" :row-key="(row: CallbackRow) => row.callbackConfigId" ref="actionRef"
        :scroll-x="tableScrollX">
        <template #tableTitle>
          <n-button type="primary" @click="handleCreate" v-permission="{ action: ['config.callback.update'] }">新建回调</n-button>
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
  import { listCallbacks, upsertCallback, setCallbackEnabled } from '@/api/config';

  const message = useMessage();
  const actionRef = ref();

  interface CallbackRow {
    callbackConfigId: number; callbackName: string; callbackType: number; targetUrl: string;
    timeoutMs: number; retryTimes: number; enabled: boolean; priority: number;
  }

  const columns = [
    { title: 'ID', key: 'callbackConfigId', width: 80 },
    { title: '名称', key: 'callbackName', width: 140 },
    { title: '类型', key: 'callbackType', width: 80 },
    { title: '目标 URL', key: 'targetUrl', width: 200, ellipsis: { tooltip: true } },
    { title: '超时(ms)', key: 'timeoutMs', width: 100 },
    { title: '重试次数', key: 'retryTimes', width: 90 },
    { title: '启用', key: 'enabled', width: 70, render: (row: CallbackRow) => h(NTag, { type: row.enabled ? 'success' : 'default' }, () => row.enabled ? '是' : '否') },
    { title: '操作', key: 'action', width: 150, render: (row: CallbackRow) => h(TableAction, { actions: [
      { label: '编辑', auth: ['config.callback.update'], onClick: () => handleEdit(row) },
      { label: row.enabled ? '禁用' : '启用', auth: ['config.callback.update'], onClick: () => toggle(row) },
    ]})},
  ];

  const loadDataTable = async (res: Record<string, unknown>) => await listCallbacks(res as Parameters<typeof listCallbacks>[0]);
  const tableScrollX = computed(() => columns.reduce((s, c) => s + ((c.width as number) || 150), 0));
  function reloadTable() { actionRef.value.reload(); }

  const schemas = [
    { field: 'callbackName', label: '名称', component: 'NInput', rules: [{ required: true }] },
    { field: 'callbackType', label: '类型', component: 'NInputNumber', defaultValue: 1 },
    { field: 'targetUrl', label: '目标 URL', component: 'NInput' },
    { field: 'rpcEndpoint', label: 'RPC Endpoint', component: 'NInput' },
    { field: 'mqExchange', label: 'MQ Exchange', component: 'NInput' },
    { field: 'mqRoutingKey', label: 'MQ Routing Key', component: 'NInput' },
    { field: 'timeoutMs', label: '超时(ms)', component: 'NInputNumber', defaultValue: 30000 },
    { field: 'retryTimes', label: '重试次数', component: 'NInputNumber', defaultValue: 3 },
    { field: 'enabled', label: '启用', component: 'NSwitch', defaultValue: true },
    { field: 'priority', label: '优先级', component: 'NInputNumber', defaultValue: 0 },
  ];
  const editingId = ref<number>(0);
  const [registerForm, { submit, setFieldsValue }] = useForm({ schemas, labelWidth: 120, layout: 'horizontal', showActionButtonGroup: false, gridProps: { cols: 1 } });
  const [modalRegister, { openModal, closeModal, setSubLoading }] = useModal({ title: '回调配置', subBtuText: '保存' });

  function handleCreate() { editingId.value = 0; openModal(); }
  function handleEdit(row: CallbackRow) { editingId.value = row.callbackConfigId; setFieldsValue(row); openModal(); }
  async function handleSubmit() {
    try { await submit(); const v = formMethods.getFieldsValue(); await upsertCallback({ ...v, callbackConfigId: editingId.value || undefined }); message.success('保存成功'); closeModal(); reloadTable(); }
    catch (e: unknown) { if (e instanceof Error) message.error(e.message); setSubLoading(false); }
  }
  async function toggle(row: CallbackRow) { await setCallbackEnabled({ callbackConfigId: row.callbackConfigId, enabled: !row.enabled }); message.success('操作成功'); reloadTable(); }
</script>
