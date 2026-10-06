<template>
  <n-drawer v-model:show="state.isDrawer" :width="width" :placement="state.placement">
    <n-drawer-content :title="title" closable>
      <n-form
        :model="formParams"
        :rules="rules"
        ref="formRef"
        label-placement="left"
        :label-width="100"
      >
        <n-form-item label="菜单标识" path="menuKey">
          <n-input placeholder="唯一标识，如 system_user" v-model:value="formParams.menuKey" />
        </n-form-item>
        <n-form-item label="菜单名称" path="menuName">
          <n-input placeholder="如 用户管理" v-model:value="formParams.menuName" />
        </n-form-item>
        <n-form-item label="类型" path="menuType">
          <n-select
            v-model:value="formParams.menuType"
            :options="[
              { label: '菜单', value: 'menu' },
              { label: '按钮', value: 'button' },
              { label: '外链 iframe', value: 'iframe' },
            ]"
          />
        </n-form-item>
        <n-form-item label="路由路径" path="routePath">
          <n-input placeholder="如 user" v-model:value="formParams.routePath" />
        </n-form-item>
        <n-form-item label="组件路径" path="componentName">
          <n-input
            placeholder="如 /system/user/user"
            v-model:value="formParams.componentName"
          />
        </n-form-item>
        <n-form-item label="图标" path="iconName">
          <n-input placeholder="如 SettingOutlined" v-model:value="formParams.iconName" />
        </n-form-item>
        <n-form-item label="权限标识" path="permissionKey">
          <n-input placeholder="如 system.user.read" v-model:value="formParams.permissionKey" />
        </n-form-item>
        <n-form-item label="排序" path="sortNo">
          <n-input-number v-model:value="formParams.sortNo" :min="0" />
        </n-form-item>
        <n-form-item label="隐藏" path="hidden">
          <n-switch v-model:value="formParams.hidden" />
        </n-form-item>
        <n-form-item label="启用" path="status">
          <n-switch v-model:value="formParams.status" />
        </n-form-item>
      </n-form>

      <template #footer>
        <n-space>
          <n-button type="primary" :loading="state.subLoading" @click="formSubmit">提交</n-button>
          <n-button @click="handleReset">重置</n-button>
        </n-space>
      </template>
    </n-drawer-content>
  </n-drawer>
</template>

<script lang="ts" setup>
  import { reactive, ref } from 'vue';
  import { useMessage } from 'naive-ui';
  import { upsertMenu } from '@/api/system/menu';

  defineProps({
    title: { type: String, default: '添加菜单' },
    width: { type: Number, default: 500 },
  });

  const emit = defineEmits(['ok']);

  const rules = {
    menuKey: { required: true, message: '请输入菜单标识', trigger: 'blur' },
    menuName: { required: true, message: '请输入菜单名称', trigger: 'blur' },
    menuType: { required: true, message: '请选择类型', trigger: 'change' },
  };

  const message = useMessage();
  const formRef = ref<{ validate: () => Promise<void> } | null>(null);

  /** 获取表单默认值 */
  const defaultForm = () => ({
    menuKey: '',
    menuName: '',
    menuType: 'menu',
    routePath: '',
    componentName: '',
    iconName: '',
    permissionKey: '',
    sortNo: 0,
    hidden: false,
    status: true,
    parentId: 0,
    menuId: 0,
  });

  const formParams = reactive(defaultForm());

  const state = reactive({
    isDrawer: false,
    subLoading: false,
    placement: 'right' as const,
  });

  function openDrawer(parentId?: number) {
    state.isDrawer = true;
    if (parentId !== undefined) {
      formParams.parentId = parentId;
    }
  }

  function closeDrawer() {
    state.isDrawer = false;
  }

  async function formSubmit() {
    try {
      await formRef.value.validate();
      state.subLoading = true;

      const payload: Record<string, unknown> = {
        menuKey: formParams.menuKey,
        menuName: formParams.menuName,
        menuType: formParams.menuType,
        routePath: formParams.routePath || undefined,
        componentName: formParams.componentName || undefined,
        iconName: formParams.iconName || undefined,
        permissionKey: formParams.permissionKey || undefined,
        sortNo: formParams.sortNo,
        hidden: formParams.hidden,
        status: formParams.status ? 1 : 0,
      };

      if (formParams.parentId) payload.parentId = formParams.parentId;
      if (formParams.menuId) payload.menuId = formParams.menuId;

      await upsertMenu(payload);
      message.success('保存成功');
      handleReset();
      closeDrawer();
      emit('ok');
    } catch (err: unknown) {
      if (err?.message) message.error(err.message);
    } finally {
      state.subLoading = false;
    }
  }

  function handleReset() {
    formRef.value?.restoreValidation();
    Object.assign(formParams, defaultForm());
  }

  /**
   * 编辑已有菜单：回填表单字段后打开 Drawer。
   * @param node — 原始菜单节点（AdminMenuNode，已 toCamelCase）
   */
  function showEdit(node: Record<string, unknown>): void {
    state.isDrawer = true;
    Object.assign(formParams, {
      menuId: node.menuId || 0,
      parentId: node.parentId || 0,
      menuKey: node.menuKey || '',
      menuName: node.menuName || '',
      menuType: node.menuType || 'menu',
      routePath: node.routePath || '',
      componentName: node.componentName || '',
      iconName: node.iconName || '',
      permissionKey: node.permissionKey || '',
      sortNo: node.sortNo ?? 0,
      hidden: node.hidden ?? false,
      status: node.status === 1,
    });
  }

  defineExpose({ openDrawer, showEdit });
</script>
