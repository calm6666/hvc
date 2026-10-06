<template>
  <div>
    <div class="n-layout-page-header"><n-card :bordered="false" title="Etcd 注册中心" /></div>
    <n-card :bordered="false" class="mt-4 proCard">
      <BasicTable :columns="columns" :request="loadDataTable" :row-key="(row: EtcdRegistryRow) => row.registryId" ref="actionRef"
        :scroll-x="tableScrollX">
        <template #tableTitle>
          <n-button type="primary" @click="handleCreate" v-permission="{ action: ['config.runtime.update'] }">新建注册中心</n-button>
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
  import { listEtcdRegistries, upsertEtcdRegistry, setEtcdRegistryEnabled } from '@/api/config';

  const message = useMessage();
  const actionRef = ref();

  interface EtcdRegistryRow { registryId: number; registryName: string; endpoints: string; serviceNamespace: string; leaseTtlSec: number; enabled: boolean; }

  const columns = [
    { title: 'ID', key: 'registryId', width: 80 },
    { title: '名称', key: 'registryName', width: 140 },
    { title: '端点', key: 'endpoints', width: 200 },
    { title: '命名空间', key: 'serviceNamespace', width: 140 },
    { title: '租约(秒)', key: 'leaseTtlSec', width: 100 },
    { title: '启用', key: 'enabled', width: 70, render: (row: EtcdRegistryRow) => h(NTag, { type: row.enabled ? 'success' : 'default' }, () => row.enabled ? '是' : '否') },
    { title: '操作', key: 'action', width: 160, render: (row: EtcdRegistryRow) => h(TableAction, { actions: [
      { label: '编辑', auth: ['config.runtime.update'], onClick: () => handleEdit(row) },
      { label: row.enabled ? '禁用' : '启用', auth: ['config.runtime.update'], onClick: () => toggle(row) },
    ]})},
  ];

  const loadDataTable = async (res: Record<string, unknown>) => await listEtcdRegistries(res as Parameters<typeof listEtcdRegistries>[0]);
  const tableScrollX = computed(() => columns.reduce((s, c) => s + ((c.width as number) || 150), 0));
  function reloadTable() { actionRef.value.reload(); }

  const schemas = [
    { field: 'registryName', label: '名称', component: 'NInput', rules: [{ required: true }] },
    { field: 'endpoints', label: '端点', component: 'NInput', defaultValue: 'http://etcd:2379' },
    { field: 'serviceNamespace', label: '命名空间', component: 'NInput', defaultValue: '/hvc' },
    { field: 'leaseTtlSec', label: '租约(秒)', component: 'NInputNumber', defaultValue: 10 },
    { field: 'dialTimeoutMs', label: '拨号超时(ms)', component: 'NInputNumber', defaultValue: 3000 },
    { field: 'enabled', label: '启用', component: 'NSwitch', defaultValue: true },
    { field: 'priority', label: '优先级', component: 'NInputNumber', defaultValue: 0 },
  ];
  const editingId = ref<number>(0);
  const [registerForm, { submit, setFieldsValue }] = useForm({ schemas, labelWidth: 110, layout: 'horizontal', showActionButtonGroup: false, gridProps: { cols: 1 } });
  const [modalRegister, { openModal, closeModal, setSubLoading }] = useModal({ title: 'Etcd 注册中心', subBtuText: '保存' });

  function handleCreate() { editingId.value = 0; openModal(); }
  function handleEdit(row: EtcdRegistryRow) { editingId.value = row.registryId; setFieldsValue(row); openModal(); }
  async function handleSubmit() {
    try { await submit(); await upsertEtcdRegistry({ ...formMethods.getFieldsValue(), registryId: editingId.value || undefined }); message.success('保存成功'); closeModal(); reloadTable(); }
    catch (e: unknown) { if (e instanceof Error) message.error(e.message); setSubLoading(false); }
  }
  async function toggle(row: EtcdRegistryRow) { await setEtcdRegistryEnabled({ registryId: row.registryId, enabled: !row.enabled }); message.success('操作成功'); reloadTable(); }
</script>
