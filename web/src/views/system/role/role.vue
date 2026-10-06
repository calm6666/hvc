<template>
  <div>
    <div class="n-layout-page-header">
      <n-card :bordered="false" title="角色管理" />
    </div>

    <n-card :bordered="false" class="mt-4 proCard">
      <BasicTable
        :columns="columns"
        :request="loadDataTable"
        :row-key="(row) => row.roleId"
        ref="actionRef"
        :actionColumn="actionColumn"
        :scroll-x="tableScrollX"
      >
        <template #tableTitle>
          <n-button
            type="primary"
            @click="addRole"
            v-permission="{ action: ['system.role.update'] }"
          >
            <template #icon>
              <n-icon><PlusOutlined /></n-icon>
            </template>
            新增角色
          </n-button>
        </template>
      </BasicTable>
    </n-card>

    <!-- 分配菜单权限弹窗 -->
    <n-modal v-model:show="showModal" :show-icon="false" preset="dialog" :title="editRoleTitle">
      <div class="py-3 menu-list">
        <n-tree
          block-line
          cascade
          checkable
          :virtual-scroll="true"
          :data="treeData"
          :expanded-keys="expandedKeys"
          :checked-keys="checkedKeys"
          style="max-height: 950px; overflow: hidden"
          @update:checked-keys="checkedTree"
          @update:expanded-keys="onExpandedKeys"
        />
      </div>
      <template #action>
        <n-space>
          <n-button type="info" ghost @click="packHandle">
            全部{{ expandedKeys.length ? '收起' : '展开' }}
          </n-button>
          <n-button type="info" ghost @click="checkedAllHandle">
            全部{{ checkedAll ? '取消' : '选择' }}
          </n-button>
          <n-button type="primary" :loading="formBtnLoading" @click="confirmForm">提交</n-button>
        </n-space>
      </template>
    </n-modal>

    <CreateModal ref="createModalRef" @ok="reloadTable" />
    <EditModal ref="editModalRef" @ok="reloadTable" />
  </div>
</template>

<script lang="ts" setup>
  import { computed, ref, reactive, h } from 'vue';
  import { useMessage } from 'naive-ui';
  import { BasicTable, TableAction } from '@/components/Table';
  import { getRoleList, getRoleMenuTree, assignRoleMenus } from '@/api/system/role';
  import { columns } from './columns';
const tableScrollX = computed(() => columns.reduce((s, c) => s + ((c.width as number) || 150), 0));
  import { PlusOutlined } from '@vicons/antd';
  import { getTreeAll } from '@/utils';
  import CreateModal from './CreateModal.vue';
  import EditModal from './EditModal.vue';

  const message = useMessage();
  const actionRef = ref();
  const createModalRef = ref();
  const editModalRef = ref();
  const showModal = ref(false);
  const formBtnLoading = ref(false);
  const checkedAll = ref(false);
  const editRoleTitle = ref('');
  const treeData = ref<any[]>([]);
  const expandedKeys = ref<string[]>([]);
  const checkedKeys = ref<string[]>([]);
  /** 当前正在分配菜单的角色 ID */
  const currentRoleId = ref<number>(0);

  /**
   * 操作列定义。
   * 行数据字段与后端 adminRoleView（toCamelCase 后）对齐：
   *   roleId, roleKey, roleName, roleDesc, status
   */
  const actionColumn = reactive({
    width: 200,
    title: '操作',
    key: 'action',
    fixed: 'right',
    render(record: Recordable) {
      return h(TableAction, {
        style: 'button',
        actions: [
          {
            label: '菜单权限',
            onClick: handleMenuAuth.bind(null, record),
            auth: ['system.role.menu_bind'],
          },
          {
            label: '编辑',
            onClick: handleEdit.bind(null, record),
            auth: ['system.role.update'],
          },
        ],
      });
    },
  });

  /** 表格数据加载函数，供 BasicTable 的 request 属性调用 */
  const loadDataTable = async (res: Record<string, unknown>) => {
    return await getRoleList(res as Parameters<typeof getRoleList>[0]);
  };

  function addRole() {
    createModalRef.value.openModal();
  }

  function reloadTable() {
    actionRef.value.reload();
  }

  /** 提交菜单权限分配 */
  async function confirmForm(e: Event) {
    e.preventDefault();
    formBtnLoading.value = true;
    try {
      // checkedKeys 在 NTree cascade 模式下可能是完整路径字符串
      // 需要取最后一段（menuId）组成数组
      const menuIds = checkedKeys.value.map((k: string) => Number(k));
      await assignRoleMenus({ roleId: currentRoleId.value, menuIds });
      message.success('菜单权限分配成功');
      showModal.value = false;
      reloadTable();
    } catch (err: unknown) {
      if (err?.message) message.error(err.message);
    } finally {
      formBtnLoading.value = false;
    }
  }

  function handleEdit(record: Recordable) {
    editModalRef.value.showModal(record);
  }

  /** 打开菜单权限分配弹窗，加载角色已绑定菜单 */
  async function handleMenuAuth(record: Recordable) {
    currentRoleId.value = record.roleId;
    editRoleTitle.value = `分配 ${record.roleName || record.roleKey} 的菜单权限`;

    try {
      const roleMenuData = await getRoleMenuTree(record.roleId);
      // data.items: 完整菜单树（adminMenuNode[]）
      // data.menuIds: 该角色已绑定的菜单 ID 列表
      treeData.value = roleMenuData?.items || [];
      checkedKeys.value = (roleMenuData?.menuIds || []).map((id: number) => String(id));
      checkedAll.value = false;
      showModal.value = true;
    } catch (err: unknown) {
      if (err?.message) message.error(err.message);
    }
  }

  function checkedTree(keys: string[]) {
    checkedKeys.value = keys;
  }

  function onExpandedKeys(keys: string[]) {
    expandedKeys.value = keys;
  }

  function packHandle() {
    if (expandedKeys.value.length) {
      expandedKeys.value = [];
    } else {
      expandedKeys.value = treeData.value.map(
        (item: { menuId?: number; key?: string }) => item.key || item.menuId?.toString()
      ) as string[];
    }
  }

  function checkedAllHandle() {
    if (!checkedAll.value) {
      checkedKeys.value = getTreeAll(treeData.value);
      checkedAll.value = true;
    } else {
      checkedKeys.value = [];
      checkedAll.value = false;
    }
  }
</script>

<style lang="less" scoped></style>
