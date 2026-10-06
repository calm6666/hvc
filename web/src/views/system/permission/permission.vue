<template>
  <div>
    <div class="n-layout-page-header"><n-card :bordered="false" title="权限管理" /></div>
    <n-card :bordered="false" class="mt-4 proCard">
      <n-data-table
        :columns="permColumns"
        :data="treeData"
        :row-key="(row: PermissionTreeNode) => row.id"
        :expanded-row-keys="expandedKeys"
        @update:expanded-row-keys="(keys) => expandedKeys = keys"
        :bordered="false"
        size="small"
        :max-height="600"
        :scroll-x="tableScrollX"
      />
    </n-card>
    <basicModal @register="modalRegister" @on-ok="handleSubmit"><BasicForm @register="registerForm" class="pt-5" /></basicModal>
  </div>
</template>

<script lang="ts" setup>
  import { h, ref, computed, onMounted } from 'vue';
  import { NButton, NTag, useMessage } from 'naive-ui';
  import { basicModal, useModal } from '@/components/Modal';
  import { BasicForm, useForm } from '@/components/Form';
  import { permissionTree, upsertPermission } from '@/api/system/permission';
  import type { PermissionTreeNode, UpsertPermissionRequest } from '@/types/api';
  import type { ItemsData } from '@/types/api/common';

  const message = useMessage();
  const treeData = ref<PermissionTreeNode[]>([]);
  const expandedKeys = ref<(string | number)[]>([]);
  const tableScrollX = computed(() => permColumns.reduce((sum, col) => sum + ((col.width as number) || 150), 0));

  const permColumns = [
    { title: '权限标识', key: 'label', width: 260, tree: true, render(row: PermissionTreeNode) {
      const tag = row.nodeType === 'module' ? h(NTag, { type: 'info', size: 'tiny' }, () => '模块') : row.nodeType === 'group' ? h(NTag, { type: 'default', size: 'tiny' }, () => '分组') : null;
      return h('span', {}, [row.label, tag ? h('span', { style: { marginLeft: '8px' } }, tag) : null, row.permission ? h('span', { style: { marginLeft: '8px', color: '#999', fontSize: '12px' } }, row.permission.permKey) : null]);
    }},
    { title: '模块', key: 'module', width: 120, render(row: PermissionTreeNode) { return row.module || '-'; }},
    { title: '权限标识', key: 'permKey', width: 200, render(row: PermissionTreeNode) { return row.permission?.permKey || '-'; }},
    { title: '描述', key: 'permDesc', width: 200, render(row: PermissionTreeNode) { return row.permission?.permDesc || '-'; }},
    { title: '操作', key: 'action', width: 100, render(row: PermissionTreeNode) {
      if (row.nodeType !== 'permission' || !row.permission) return '';
      return h(NButton, { size: 'tiny', onClick: () => handleEdit(row) }, () => '编辑');
    }},
  ];

  const schemas = [
    { field: 'permKey', label: '权限标识', component: 'NInput', rules: [{ required: true }] },
    { field: 'permName', label: '权限名称', component: 'NInput', rules: [{ required: true }] },
    { field: 'module', label: '所属模块', component: 'NInput', rules: [{ required: true }] },
    { field: 'permDesc', label: '描述', component: 'NInput' },
  ];
  const [registerForm, { submit, getFieldsValue, setFieldsValue }] = useForm({ schemas, labelWidth: 100, layout: 'horizontal', showActionButtonGroup: false, gridProps: { cols: 1 } });
  const [modalRegister, { openModal, closeModal, setSubLoading }] = useModal({ title: '权限点', subBtuText: '保存' });

  function handleEdit(row: PermissionTreeNode): void {
    const p = row.permission;
    setFieldsValue({ permKey: p?.permKey || row.key || '', permName: p?.permName || row.label || '', module: p?.module || row.module || '', permDesc: p?.permDesc || '' });
    openModal();
  }
  async function handleSubmit(): Promise<void> {
    try { await submit(); await upsertPermission(getFieldsValue() as UpsertPermissionRequest); message.success('保存成功'); closeModal(); loadTree(); }
    catch (e: unknown) { if (e instanceof Error) message.error(e.message); setSubLoading(false); }
  }

  function collectAllPermKeys(nodes: PermissionTreeNode[]): (string | number)[] {
    const keys: (string | number)[] = [];
    for (const n of nodes) { if (n.children?.length) { keys.push(n.id); keys.push(...collectAllPermKeys(n.children)); } }
    return keys;
  }

  async function loadTree(): Promise<void> {
    try { const data = await permissionTree() as ItemsData<PermissionTreeNode>; treeData.value = data?.items || []; }
    catch (e) { console.error(e); }
  }
  onMounted(() => loadTree());
</script>
