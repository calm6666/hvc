<template>
  <div>
    <div class="n-layout-page-header">
      <n-card :bordered="false" title="直播频道管理" />
    </div>

    <n-card :bordered="false" class="mt-4 proCard">
      <!--
        直播频道列表。
        使用 BasicTable 封装组件，通过 :request 绑定分页接口，
        通过 :actionColumn 渲染每行的操作按钮。
        字段与后端 LiveChannel 结构体（经 toCamelCase 转换后）对齐。
      -->
      <BasicTable
        :columns="columns"
        :request="loadDataTable"
        :row-key="(row: LiveChannelRow) => row.channelId"
        ref="actionRef"
        :actionColumn="actionColumn"
        :scroll-x="tableScrollX"
      >
        <template #tableTitle>
          <n-button
            type="primary"
            @click="handleCreate"
            v-permission="{ action: ['live.channel.create'] }"
          >
            创建频道
          </n-button>
        </template>
      </BasicTable>
    </n-card>

    <!--
      新建频道弹窗（使用 basicModal + BasicForm 封装组件）。
      modalRegister 将 Modal 实例注册到 useModal hook，
      通过 openModal / closeModal 控制显隐，
      通过 handleSubmit 处理表单提交。
    -->
    <basicModal @register="modalRegister" @on-ok="handleSubmit">
      <BasicForm @register="registerForm" class="pt-5" />
    </basicModal>
    <!-- 编辑频道弹窗 -->
    <basicModal @register="editModalRegister" @on-ok="handleEditSubmit">
      <BasicForm @register="editFormRegister" class="pt-5" />
    </basicModal>
  </div>
</template>

<script lang="ts" setup>
  import { computed, h, ref, reactive } from 'vue';
  import { NTag, useMessage } from 'naive-ui';
  import { BasicTable, TableAction } from '@/components/Table';
  import { basicModal, useModal } from '@/components/Modal';
  import { BasicForm, useForm } from '@/components/Form';
  import { 
    listChannels,
    createChannel,
    startChannel,
    stopChannel,
    deleteChannel,
  } from '@/api/live';

  const message = useMessage();

  /** 表格组件引用，通过 actionRef.value.reload() 刷新列表 */
  const actionRef = ref();

  // ============================================================
  // 数据类型定义
  // ============================================================

  /**
   * 直播频道行数据类型。
   * 字段与后端 model.LiveChannel（经 HTTP 拦截器 toCamelCase 转换后）对齐：
   *   ChannelID → channelId
   *   ChannelKey → channelKey
   *   ChannelName → channelName
   *   Status → status          // "active" | "idle" | "stopped"
   *   PlayDomain → playDomain
   *   PushDomain → pushDomain
   *   ProfileID → profileId
   *   CreatedAt → createdAt
   */
  interface LiveChannelRow {
    channelId: number;
    channelKey: string;
    channelName: string;
    status: string;
    playDomain: string;
    pushDomain: string;
    profileId: number;
    createdAt: string;
  }

  // ============================================================
  // 表格列定义
  // ============================================================

  const columns = [
    { title: 'ID', key: 'channelId', width: 80 },
    { title: '频道标识', key: 'channelKey', width: 140 },
    { title: '频道名', key: 'channelName', width: 140 },
    {
      title: '状态',
      key: 'status',
      width: 100,
      /**
       * 频道状态 — 颜色 + 文字映射。
       * active（推流中）→ 绿色，idle（空闲）→ 灰色，stopped（已停止）→ 黄色。
       */
      render: (row: LiveChannelRow) => {
        const statusMap: Record<
          string,
          { type: 'success' | 'default' | 'warning'; label: string }
        > = {
          active: { type: 'success', label: '推流中' },
          idle: { type: 'default', label: '空闲' },
          stopped: { type: 'warning', label: '已停止' },
        };
        const info = statusMap[row.status] || {
          type: 'default' as const,
          label: row.status,
        };
        return h(NTag, { type: info.type }, () => info.label);
      },
    },
    { title: '推流域名', key: 'pushDomain', width: 160 },
    { title: '播放域名', key: 'playDomain', width: 160 },
    { title: '转码配置 ID', key: 'profileId', width: 110 },
    { title: '创建时间', key: 'createdAt', width: 170 },
  ];

  /**
   * 操作列定义。
   * 根据频道当前状态动态显示按钮：
   *   - 非推流中 → 显示"开始"
   *   - 推流中   → 显示"停止"（带二次确认）
   *   - 任何状态 → 显示"删除"（带二次确认）
   * 按钮的 auth 属性控制权限可见性（与后端 RBAC permission_key 对应）。
   */
  const actionColumn = reactive({
    width: 210,
    title: '操作',
    key: 'action',
    fixed: 'right' as const,
    render(record: LiveChannelRow) {
      return h(TableAction, {
        style: 'button',
        actions: [
          {
            label: '编辑',
            auth: ['live.channel.update'],
            onClick: () => handleEdit(record),
          },
          {
            label: '开始',
            auth: ['live.channel.start'],
            ifShow: () => record.status !== 'active',
            onClick: () => handleStart(record),
          },
          {
            label: '停止',
            auth: ['live.channel.stop'],
            ifShow: () => record.status === 'active',
            popConfirm: {
              title: '确认停止推流?',
              confirm: () => handleStop(record),
            },
          },
          {
            label: '删除',
            auth: ['live.channel.delete'],
            popConfirm: {
              title: '确认删除此频道?',
              confirm: () => handleDel(record),
            },
          },
        ],
      });
    },
  });

  // ============================================================
  // 表格数据加载
  // ============================================================

  /**
   * 表格数据加载函数，供 BasicTable 的 :request 属性调用。
   * 参数 res 由 BasicTable 的内部分页逻辑自动组装（page, pageSize 等），
   * 直接透传给后端分页接口。
   */
  const loadDataTable = async (res: Record<string, unknown>) =>
    await listChannels(res as Parameters<typeof listChannels>[0]);
  const tableScrollX = computed(() => columns.reduce((s, c) => s + ((c.width as number) || 150), 0));

  /** 刷新表格数据 */
  function reloadTable(): void {
    actionRef.value.reload();
  }

  // ============================================================
  // 新建频道弹窗
  // ============================================================

  /**
   * 新建频道表单 schema。
   * 字段经 HTTP 拦截器 toSnakeCase 转换后发送到后端：
   *   channelKey → channel_key
   *   channelName → channel_name
   *   profileId → profile_id
   */
  const schemas = [
    {
      field: 'channelKey',
      label: '频道标识',
      component: 'NInput',
      rules: [{ required: true, message: '请输入频道标识' }],
    },
    {
      field: 'channelName',
      label: '频道名称',
      component: 'NInput',
      rules: [{ required: true, message: '请输入频道名称' }],
    },
    {
      field: 'profileId',
      label: '转码配置 ID',
      component: 'NInputNumber',
      defaultValue: 1,
    },
  ];

  const [registerForm, { submit, setFieldsValue }] = useForm({
    schemas,
    labelWidth: 100,
    layout: 'horizontal',
    showActionButtonGroup: false,
    gridProps: { cols: 1 },
  });

  const [modalRegister, { openModal, closeModal, setSubLoading }] = useModal({
    title: '创建频道',
    subBtuText: '确定',
  });

  /** 打开新建频道弹窗 */
  function handleCreate(): void {
    openModal();
  }

  /**
   * 提交新建频道表单。
   * 先通过 submit() 触发 BasicForm 内部的 Naive UI 表单校验，
   * 校验通过后再调用 createChannel API。
   */
  async function handleSubmit(): Promise<void> {
    try {
      const values = await submit();
      if (!values) {
        setSubLoading(false);
        return;
      }
      await createChannel(
        values as { channelKey: string; channelName: string; profileId: number }
      );
      message.success('频道创建成功');
      closeModal();
      reloadTable();
    } catch (e: unknown) {
      if (e instanceof Error) message.error(e.message);
      setSubLoading(false);
    }
  }

  // ============================================================
  // 频道操作
  // ============================================================

  /**
   * 启动频道（开始拉流转码）。
   * POST /v1/admin/live/channel/start
   * 权限: live.channel.start
   */
  async function handleStart(row: LiveChannelRow): Promise<void> {
    await startChannel(row.channelId);
    message.success('已发起开播');
    reloadTable();
  }

  /**
   * 停止频道。
   * POST /v1/admin/live/channel/stop
   * 权限: live.channel.stop
   */
  async function handleStop(row: LiveChannelRow): Promise<void> {
    await stopChannel(row.channelId);
    message.success('已停止');
    reloadTable();
  }

  /**
   * 删除频道。
   * POST /v1/admin/live/channel/delete
   * 权限: live.channel.delete
   */
  async function handleDel(row: LiveChannelRow): Promise<void> {
    await deleteChannel(row.channelId);
    message.success('已删除');
    reloadTable();
  }

  // ============================================================
  // 编辑频道弹窗
  // ============================================================

  const editingChannelId = ref<number>(0);
  const editSchemas = [
    { field: 'channelName', label: '频道名称', component: 'NInput' },
    { field: 'profileId', label: '转码配置 ID', component: 'NInputNumber' },
    { field: 'enableSourceRendition', label: '启用原始画质', component: 'NSwitch', defaultValue: false },
    { field: 'enableWatermark', label: '启用水印', component: 'NSwitch', defaultValue: false },
    { field: 'playDomain', label: '播放域名', component: 'NInput' },
    { field: 'pushDomain', label: '推流域名', component: 'NInput' },
  ];
  const [editFormRegister, { submit: editSubmit, setFieldsValue: editSetFields }] = useForm({ schemas: editSchemas, labelWidth: 120, layout: 'horizontal', showActionButtonGroup: false, gridProps: { cols: 1 } });
  const [editModalRegister, { openModal: openEditModal, closeModal: closeEditModal, setSubLoading: setEditSubLoading }] = useModal({ title: '编辑频道', subBtuText: '保存' });

  /** 编辑频道：打开弹窗并回填当前值 */
  function handleEdit(row: LiveChannelRow): void {
    editingChannelId.value = row.channelId;
    editSetFields({ channelName: row.channelName, profileId: row.profileId, enableSourceRendition: row.enableSourceRendition, enableWatermark: row.enableWatermark, playDomain: row.playDomain || '', pushDomain: row.pushDomain || '' });
    openEditModal();
  }

  /** 提交编辑 */
  async function handleEditSubmit(): Promise<void> {
    try {
      const v = await editSubmit();
      if (!v) { setEditSubLoading(false); return; }
      await updateChannel({ channelId: editingChannelId.value, channelName: v.channelName as string, profileId: v.profileId as number, enableSourceRendition: v.enableSourceRendition as boolean, enableWatermark: v.enableWatermark as boolean, playDomain: v.playDomain as string, pushDomain: v.pushDomain as string });
      message.success('频道更新成功'); closeEditModal(); reloadTable();
    } catch (e: unknown) { if (e instanceof Error) message.error(e.message); setEditSubLoading(false); }
  }
</script>
