<template>
  <div>
    <div class="n-layout-page-header">
      <n-card :bordered="false" title="菜单管理">
        <template #header-extra>
          <n-space>
            <n-dropdown trigger="hover" @select="selectAddMenu" :options="addMenuOptions">
              <n-button type="primary" size="small" v-permission="{ action: ['system.menu.update'] }">添加菜单</n-button>
            </n-dropdown>
          </n-space>
        </template>
      </n-card>
    </div>
    <n-card :bordered="false" class="mt-4 proCard">
      <!-- NDataTable 原生树形模式：data 中有 children 字段即自动可展开 -->
      <n-data-table
        :columns="tableColumns"
        :data="treeData"
        :row-key="(row: AdminMenuNode) => row.menuId"
        :expanded-row-keys="expandedKeys"
        @update:expanded-row-keys="(keys) => expandedKeys = keys"
        :bordered="false"
        size="small"
        :max-height="600"
        :scroll-x="tableScrollX"
      />
    </n-card>
    <CreateDrawer ref="createDrawerRef" :title="drawerTitle" @ok="loadTree" />
  </div>
</template>

<script lang="ts" setup>
  import { h, ref, reactive, computed, onMounted } from 'vue';
  import { NButton, NTag, useMessage, useDialog } from 'naive-ui';
  import { getMenuTree, deleteMenu } from '@/api/system/menu';
  import type { AdminMenuNode } from '@/types/api';
  import type { ItemsData } from '@/types/api/common';
  import CreateDrawer from './CreateDrawer.vue';

  const message = useMessage();
  const dialog = useDialog();
  const createDrawerRef = ref<{ openDrawer: (parentId?: number) => void; showEdit: (node: Record<string, unknown>) => void } | null>(null);

  /** 菜单树数据（NDataTable 自动识别 children 字段渲染为可展开树行） */
  const treeData = ref<AdminMenuNode[]>([]);
  /** 受控展开状态（初始全部展开） */
  const expandedKeys = ref<(string | number)[]>([]);
  const selectedParentId = ref<number>(0);
  /** 横向滚动宽度 — 从列定义动态计算 */
  const tableScrollX = computed(() => tableColumns.reduce((sum, col) => sum + ((col.width as number) || 150), 0));
  const drawerTitle = ref('添加菜单');
  const addMenuOptions = [
    { label: '添加顶级菜单', key: 'home' },
    { label: '添加子菜单（需先选中父级）', key: 'son' },
  ];

  const tableColumns = [
    { title: '菜单名称', key: 'menuName', width: 240, tree: true },
    { title: '标识', key: 'menuKey', width: 160 },
    { title: '类型', key: 'menuType', width: 80, render(row: AdminMenuNode) { const m: Record<string, string> = { menu: '菜单', button: '按钮', iframe: '外链' }; return m[row.menuType] || row.menuType; }},
    { title: '路由路径', key: 'routePath', width: 150 },
    { title: '组件路径', key: 'component', width: 200, ellipsis: { tooltip: true } },
    { title: '图标', key: 'iconName', width: 150 },
    { title: '权限标识', key: 'permissionKey', width: 180 },
    { title: '排序', key: 'sortNo', width: 60, align: 'center' as const },
    { title: '状态', key: 'status', width: 70, render(row: AdminMenuNode) { return h(NTag, { type: row.status === 1 ? 'success' : 'default', size: 'small' }, () => row.status === 1 ? '启用' : '禁用'); }},
    { title: '操作', key: 'action', width: 130, render(row: AdminMenuNode) { return h('div', { style: { display: 'flex', gap: '8px' } }, [h(NButton, { size: 'tiny', onClick: () => handleEdit(row) }, () => '编辑'), h(NButton, { size: 'tiny', type: 'error', onClick: () => handleDel(row) }, () => '删除')]); }},
  ];

  function selectAddMenu(key: string): void {
    drawerTitle.value = key === 'home' ? '添加顶级菜单' : '添加子菜单';
    const parentId = key === 'home' ? 0 : selectedParentId.value;
    createDrawerRef.value?.openDrawer(parentId);
  }
  function handleEdit(row: AdminMenuNode): void { selectedParentId.value = row.menuId; createDrawerRef.value?.showEdit(row); }
  function handleDel(row: AdminMenuNode): void {
    dialog.warning({ title: '确认删除', content: `确定要删除「${row.menuName}」吗？`, positiveText: '确定', negativeText: '取消',
      onPositiveClick: async () => { try { await deleteMenu({ menuId: row.menuId }); message.success('删除成功'); loadTree(); } catch (e: unknown) { if (e instanceof Error) message.error(e.message); } },
    });
  }

  function collectAllKeys(nodes: AdminMenuNode[]): (string | number)[] {
    const keys: (string | number)[] = [];
    for (const n of nodes) { if (n.children?.length) { keys.push(n.menuId); keys.push(...collectAllKeys(n.children)); } }
    return keys;
  }

  async function loadTree(): Promise<void> {
    try { const data = await getMenuTree() as ItemsData<AdminMenuNode>; treeData.value = data?.items || []; }
    catch (e) { console.error('加载菜单树失败', e); }
  }

  onMounted(() => loadTree());
</script>
